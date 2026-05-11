package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/ingestor/src/models"
	"github.com/omnigate/services/ingestor/src/repository"
	"go.opentelemetry.io/otel/propagation"
)

// HandleIngest accepts events from any content type:
//   - multipart/form-data : existing behaviour + fallback to collected form fields
//   - application/json    : raw body as payload; trusted workers may send an
//                           envelope {source_id, transaction_id, payload}
//   - everything else     : raw body stored verbatim as payload
func HandleIngest(c *gin.Context) {
	defer c.Request.Body.Close()

	// Auth headers injected by NGINX after validating the upstream request.
	headerSourceID := c.GetHeader("X-Source-ID")
	sourceName := c.GetHeader("X-Source-Name")
	gateID := c.GetHeader("X-Gate-ID")
	permissions := c.GetHeader("X-Permissions")
	slog.Info("Headers", "source_id", headerSourceID, "source_name", sourceName,
		"gate_id", gateID, "permissions", permissions)

	if headerSourceID == "" || gateID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication headers"})
		return
	}

	canAssumeSource := strings.Contains(permissions, "ingest:assume-source")

	// ── Body parsing ─────────────────────────────────────────────────────────
	// Fields extracted from the body; source assumption is applied afterwards.
	var (
		payload         string
		assumedSourceID string
		assumedGateID   string
		transactionID   string
	)

	ct := c.ContentType() // base MIME type, no params (Gin strips them)

	if ct == "multipart/form-data" || ct == "application/x-www-form-urlencoded" {
		assumedSourceID = c.PostForm("source_id")
		assumedGateID = c.PostForm("gate_id")
		transactionID = c.PostForm("transaction_id")

		payload = c.PostForm("payload")
		if payload == "" {
			// No explicit payload field — serialise all other text fields to JSON.
			var formErr error
			if ct == "multipart/form-data" {
				formErr = c.Request.ParseMultipartForm(32 << 20)
			} else {
				formErr = c.Request.ParseForm()
			}
			if formErr == nil {
				fields := make(map[string]any)
				for k, vs := range c.Request.PostForm {
					if k != "source_id" && k != "gate_id" && k != "transaction_id" && len(vs) > 0 {
						fields[k] = vs[0]
					}
				}
				if len(fields) > 0 {
					if b, err := json.Marshal(fields); err == nil {
						payload = string(b)
					}
				}
			}
		}
	} else {
		// JSON, XML, text/plain, or any other raw body.
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20))
		if err != nil {
			slog.Error("Failed to read request body", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		payload = string(body)

		// For JSON from a trusted worker (ingest:assume-source) check for the
		// Puller envelope: {source_id, transaction_id, payload}.
		if ct == "application/json" && canAssumeSource && len(body) > 0 {
			var env struct {
				SourceID      string          `json:"source_id"`
				GateID        string          `json:"gate_id"`
				TransactionID string          `json:"transaction_id"`
				Payload       json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(body, &env); err == nil && env.SourceID != "" {
				assumedSourceID = env.SourceID
				assumedGateID = env.GateID
				transactionID = env.TransactionID
				if env.Payload != nil {
					payload = string(env.Payload)
				}
			}
		}
	}

	// ── Source identity resolution ────────────────────────────────────────────
	sourceID := headerSourceID
	if canAssumeSource && assumedSourceID != "" {
		slog.Info("Assuming source identity from request body",
			"puller_source_id", headerSourceID,
			"assumed_source_id", assumedSourceID,
			"assumed_gate_id", assumedGateID,
		)
		sourceID = assumedSourceID
		sourceName = "assumed:" + assumedSourceID
		if assumedGateID != "" {
			gateID = assumedGateID
		} else if assumedConfig, err := repository.GetDeviceConfig(assumedSourceID); err == nil {
			gateID = assumedConfig.GateID
		}
	}

	var txnIDPtr *string
	if transactionID != "" {
		txnIDPtr = &transactionID
	}

	slog.Info("Ingest event", "source_id", sourceID, "gate_id", gateID,
		"transaction_id", transactionID, "content_type", ct)

	// ── Device config (trigger metadata) ─────────────────────────────────────
	triggerEnabled := false
	var triggerURL *string
	var triggerSourceID *string
	config, err := repository.GetDeviceConfig(headerSourceID)
	if err == nil {
		triggerEnabled = config.TriggerEnabled
		triggerURL = config.TriggerURL
		triggerSourceID = config.TriggerSourceID
	} else {
		slog.Warn("Failed to fetch device config", "source_id", headerSourceID, "error", err)
	}

	now := time.Now().UTC()

	// ── S3: raw payload ───────────────────────────────────────────────────────
	rawKey := fmt.Sprintf("raw/%s/%04d/%02d/%02d/%s.json",
		gateID, now.Year(), now.Month(), now.Day(), uuid.New().String())

	if err := repository.UploadToS3(rawKey, []byte(payload)); err != nil {
		slog.Error("Failed to upload RAW payload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Storage error"})
		return
	}

	// ── S3: image (multipart only) ────────────────────────────────────────────
	// c.FormFile works after ParseMultipartForm, which was already called above.
	var imageKeys []string
	if ct == "multipart/form-data" {
		file, err := c.FormFile("image")
		if err == nil && file != nil {
			imageKey := fmt.Sprintf("images/%s/%04d/%02d/%02d/%s.jpg",
				gateID, now.Year(), now.Month(), now.Day(), uuid.New().String())
			if err := repository.UploadFileToS3(file, imageKey); err != nil {
				slog.Error("Failed to upload image", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload failed"})
				return
			}
			imageKeys = append(imageKeys, imageKey)
		} else if err != nil && err != http.ErrMissingFile {
			slog.Warn("Failed to read image field", "error", err)
		}
	}
	if imageKeys == nil {
		imageKeys = []string{}
	}

	// ── Publish to Valkey Stream ──────────────────────────────────────────────
	event := &models.IngestEvent{
		SourceID:      sourceID,
		SourceName:    sourceName,
		GateID:        gateID,
		Payload:       payload,
		RawStorageKey: rawKey,
		ImageKeys:     imageKeys,
		TransactionID: txnIDPtr,
		Timestamp:     now,
	}

	tc := propagation.TraceContext{}
	carrier := propagation.MapCarrier{}
	tc.Inject(c.Request.Context(), carrier)
	event.TraceContext = carrier.Get("traceparent")

	if err := repository.PublishToStream("events:adapter", event.ToJSON()); err != nil {
		slog.Error("Failed to publish event", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Event publish failed"})
		return
	}

	slog.Info("Event published", "source_id", sourceID, "raw_key", rawKey)

	response := gin.H{
		"event_id":          rawKey,
		"transaction_id":    transactionID,
		"trigger_enabled":   triggerEnabled,
		"trigger_url":       triggerURL,
		"trigger_source_id": triggerSourceID,
	}
	if len(imageKeys) > 0 {
		response["image_key"] = imageKeys[0]
	}

	c.JSON(http.StatusAccepted, response)
}
