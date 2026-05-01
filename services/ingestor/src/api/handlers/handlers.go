package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/ingestor/src/models"
	"github.com/omnigate/services/ingestor/src/repository"
)

// HandleIngest processes incoming events (with or without an image)
func HandleIngest(c *gin.Context) {
	// Extract auth headers (set by NGINX from AUTH service)
	headerSourceID := c.GetHeader("X-Source-ID")
	sourceName := c.GetHeader("X-Source-Name")
	gateID := c.GetHeader("X-Gate-ID")
	permissions := c.GetHeader("X-Permissions")

	if headerSourceID == "" || gateID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication headers"})
		return
	}

	// Determine effective source ID:
	// If caller has `ingest:assume-source` permission (i.e., it's the Puller),
	// allow overriding source_id from the request body.
	sourceID := headerSourceID
	canAssumeSource := strings.Contains(permissions, "ingest:assume-source")
	if canAssumeSource {
		if bodySourceID := c.PostForm("source_id"); bodySourceID != "" {
			slog.Info("Assuming source identity from request body",
				"puller_source_id", headerSourceID,
				"assumed_source_id", bodySourceID,
			)
			sourceID = bodySourceID
			// Recalculate gateID from the assumed device's config
			if assumedConfig, err := repository.GetDeviceConfig(bodySourceID); err == nil {
				gateID = assumedConfig.GateID
				sourceName = "assumed:" + bodySourceID
			}
		}
	}

	// Get optional payload (metadata)
	payload := c.PostForm("payload")
	if payload == "" {
		payload = "{}" // Empty JSON if not provided
	}

	// Optional: transaction_id from Puller or other sources
	transactionID := c.PostForm("transaction_id")
	if transactionID == "" {
		txID, err := repository.GetOrCreateTransaction(gateID)
		if err != nil {
			slog.Error("Failed to get/create transaction", "error", err)
		} else {
			transactionID = txID
		}
	}

	var txnIDPtr *string
	if transactionID != "" {
		txnIDPtr = &transactionID
	}

	// Fetch device config for trigger info (using the original header source ID — the actual device)
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

	slog.Info("Ingest event", "source_id", sourceID, "gate_id", gateID, "transaction_id", transactionID)

	// 1. Upload RAW payload to S3
	now := time.Now()
	rawKey := fmt.Sprintf("raw/%s/%04d/%02d/%02d/%s.json",
		gateID, now.Year(), now.Month(), now.Day(), uuid.New().String())

	if err := repository.UploadToS3(rawKey, []byte(payload)); err != nil {
		slog.Error("Failed to upload RAW payload", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Storage error"})
		return
	}

	// 2. Check for image and upload if present
	var imageKeys []string
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
	} else if err != http.ErrMissingFile {
		// Log if there's an error other than "no file provided"
		slog.Warn("Failed to read image field", "error", err)
	}

	if imageKeys == nil {
		imageKeys = []string{}
	}

	// 3. Create event for stream
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

	// 4. Publish to Valkey Stream
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


