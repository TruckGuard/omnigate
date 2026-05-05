package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/logic"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/datatypes"
)

type CreateEventRequest struct {
	EventTypeID   uuid.UUID      `json:"event_type_id" binding:"required"`
	GateID        string         `json:"gate_id" binding:"required"`
	SourceID      string         `json:"source_id" binding:"required"`
	Data          datatypes.JSON `json:"data" binding:"required"`
	RawPayload    string         `json:"raw_payload"`
	RawDataKey    string         `json:"raw_data_key"`
	ImageKeys     []string       `json:"image_keys"`
	TransactionID *uuid.UUID     `json:"transaction_id"` // Optional, from Puller
}

func HandleCreateEvent(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Enrich span with business context
	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		attribute.String("truckguard.gate_id", req.GateID),
		attribute.String("truckguard.source_id", req.SourceID),
	)

	// Validate event data against event type schema
	eventType := repository.GetEventType(req.EventTypeID)
	if eventType == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event_type_id"})
		return
	}

	if err := validateEventData(req.Data, eventType.Fields); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data: " + err.Error()})
		return
	}

	// Determine transaction
	var transactionID uuid.UUID
	if req.TransactionID != nil {
		// Transaction provided by Puller
		transactionID = *req.TransactionID
	} else {
		transactionID = logic.FindOrCreateTransaction(req.GateID)
		// Enforce max events per transaction
		if max := logic.MaxEventsForGate(req.GateID); max > 0 {
			if repository.CountEventsForTransaction(transactionID) >= int64(max) {
				repository.RDB.Del(context.Background(), logic.ActiveTxKey(req.GateID))
				transactionID = logic.FindOrCreateTransaction(req.GateID)
			}
		}
	}

	span.SetAttributes(attribute.String("truckguard.transaction_id", transactionID.String()))

	imgBytes, _ := json.Marshal(req.ImageKeys)

	// Create event
	event := &models.Event{
		TransactionID: &transactionID,
		EventTypeID:   req.EventTypeID,
		GateID:        req.GateID,
		SourceID:      req.SourceID,
		Data:          req.Data,
		RawPayload:    req.RawPayload,
		RawDataKey:    req.RawDataKey,
		ImageKeys:     datatypes.JSON(imgBytes),
	}

	savedEvent := repository.CreateEvent(event)

	c.JSON(http.StatusCreated, gin.H{
		"event":          savedEvent,
		"transaction_id": transactionID,
	})
}

func HandleListEvents(c *gin.Context) {
	transactionID := c.Query("transaction_id")
	gateID := c.Query("gate_id")
	sourceID := c.Query("source_id")

	events := repository.ListEvents(transactionID, gateID, sourceID)
	c.JSON(http.StatusOK, events)
}

func HandleGetEvent(c *gin.Context) {
	id := c.Param("id")
	eventID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	event := repository.GetEvent(eventID)
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}

func HandleDeleteEvent(c *gin.Context) {
	id := c.Param("id")
	eventID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid event ID"})
		return
	}

	if err := repository.DeleteEvent(eventID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event deleted"})
}

func HandleGetLatestEventForSource(c *gin.Context) {
	sourceID := c.Query("source_id")
	if sourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_id is required"})
		return
	}
	event := repository.GetLatestEventForSource(sourceID)
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No events found for this source"})
		return
	}
	c.JSON(http.StatusOK, event)
}

// validateEventData validates event data against type schema
func validateEventData(data datatypes.JSON, schema datatypes.JSON) error {
	// Simple stub for json schema validation
	return nil
}
