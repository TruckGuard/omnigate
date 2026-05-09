package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
	"gorm.io/datatypes"
)

// HandleTriggerDevice queues a Puller task for every trigger configured on the device.
// The Puller resolves the pull URL by fetching the target device's own TriggerURL.
func HandleTriggerDevice(c *gin.Context) {
	id := c.Param("id")
	configID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.DeviceConfig
	if err := repository.DB.First(&config, "id = ?", configID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	if !config.TriggerEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Trigger not enabled for this device"})
		return
	}

	var triggers []models.Trigger
	if config.Triggers != nil {
		json.Unmarshal(config.Triggers, &triggers) //nolint:errcheck
	}

	queued := 0
	for _, t := range triggers {
		if t.SourceID == "" {
			continue
		}
		msg := map[string]any{
			"trigger_source_id": t.SourceID,
			"gate_id":           config.GateID,
			"source_id":         config.SourceID,
			"transaction_id":    "",
			"context":           map[string]any{},
		}
		data, _ := json.Marshal(msg)
		if err := repository.PublishToPuller(string(data)); err == nil {
			queued++
		}
	}

	if queued == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No triggers configured or all failed to queue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%d trigger(s) queued", queued)})
}

func HandleListDeviceConfigs(c *gin.Context) {
	configs := repository.ListDeviceConfigs()
	c.JSON(http.StatusOK, configs)
}

func HandleGetDeviceConfig(c *gin.Context) {
	param := c.Param("source_id")
	var config *models.DeviceConfig
	if id, err := uuid.Parse(param); err == nil {
		config = repository.GetDeviceConfigByID(id)
	} else {
		config = repository.GetDeviceConfigBySourceID(param)
	}
	if config == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}
	c.JSON(http.StatusOK, config)
}

func HandleCreateDeviceConfig(c *gin.Context) {
	var req struct {
		SourceID       string         `json:"source_id" binding:"required"`
		EventTypeID    uuid.UUID      `json:"event_type_id" binding:"required"`
		GateID         string         `json:"gate_id" binding:"required"`
		DataMapping    datatypes.JSON `json:"data_mapping" binding:"required"`
		DataType       string         `json:"data_type" binding:"required"`
		TriggerURL     *string        `json:"trigger_url"`
		Triggers       datatypes.JSON `json:"triggers"`
		TriggerEnabled bool           `json:"trigger_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	triggers := req.Triggers
	if triggers == nil {
		triggers = datatypes.JSON([]byte("[]"))
	}

	config := &models.DeviceConfig{
		SourceID:       req.SourceID,
		EventTypeID:    req.EventTypeID,
		GateID:         req.GateID,
		DataMapping:    req.DataMapping,
		DataType:       req.DataType,
		TriggerURL:     req.TriggerURL,
		Triggers:       triggers,
		TriggerEnabled: req.TriggerEnabled,
		Enabled:        true,
	}

	savedConfig := repository.CreateDeviceConfig(config)
	c.JSON(http.StatusCreated, savedConfig)
}

func HandleUpdateDeviceConfig(c *gin.Context) {
	id := c.Param("id")
	configID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	var config models.DeviceConfig
	if err := repository.DB.First(&config, "id = ?", configID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}

	var req struct {
		EventTypeID    *uuid.UUID       `json:"event_type_id"`
		GateID         *string          `json:"gate_id"`
		DataType       *string          `json:"data_type"`
		DataMapping    *json.RawMessage `json:"data_mapping"`
		TriggerURL     *string          `json:"trigger_url"`
		TriggerEnabled *bool            `json:"trigger_enabled"`
		Triggers       *json.RawMessage `json:"triggers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.EventTypeID != nil {
		config.EventTypeID = *req.EventTypeID
	}
	if req.GateID != nil {
		config.GateID = *req.GateID
	}
	if req.DataType != nil {
		config.DataType = *req.DataType
	}
	if req.DataMapping != nil {
		config.DataMapping = datatypes.JSON(*req.DataMapping)
	}
	if req.TriggerURL != nil {
		config.TriggerURL = req.TriggerURL
	}
	if req.TriggerEnabled != nil {
		config.TriggerEnabled = *req.TriggerEnabled
	}
	if req.Triggers != nil {
		config.Triggers = datatypes.JSON(*req.Triggers)
	}

	if err := repository.UpdateDeviceConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

func HandleDeleteDeviceConfig(c *gin.Context) {
	id := c.Param("id")
	configID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	if err := repository.DeleteDeviceConfig(configID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device Config deleted"})
}
