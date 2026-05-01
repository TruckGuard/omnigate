package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/logic"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
	"gorm.io/datatypes"
)

type CreateEventRequest struct {
	EventTypeID   uuid.UUID      `json:"event_type_id" binding:"required"`
	GateID        string         `json:"gate_id" binding:"required"`
	SourceID      string         `json:"source_id" binding:"required"`
	Data          datatypes.JSON `json:"data" binding:"required"`
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
		// Find or create transaction for this gate
		transactionID = logic.FindOrCreateTransaction(req.GateID)
	}

	imgBytes, _ := json.Marshal(req.ImageKeys)

	// Create event
	event := &models.Event{
		TransactionID: &transactionID,
		EventTypeID:   req.EventTypeID,
		GateID:        req.GateID,
		SourceID:      req.SourceID,
		Data:          req.Data,
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

// validateEventData validates event data against type schema
func validateEventData(data datatypes.JSON, schema datatypes.JSON) error {
	// Simple stub for json schema validation
	return nil
}
