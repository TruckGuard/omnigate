package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
	"gorm.io/datatypes"
)

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
		SourceID        string         `json:"source_id" binding:"required"`
		EventTypeID     uuid.UUID      `json:"event_type_id" binding:"required"`
		GateID          string         `json:"gate_id" binding:"required"`
		DataMapping     datatypes.JSON `json:"data_mapping" binding:"required"`
		DataType        string         `json:"data_type" binding:"required"`
		TriggerURL      *string        `json:"trigger_url"`
		TriggerSourceID *string        `json:"trigger_source_id"`
		TriggerEnabled  bool           `json:"trigger_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := &models.DeviceConfig{
		SourceID:        req.SourceID,
		EventTypeID:     req.EventTypeID,
		GateID:          req.GateID,
		DataMapping:     req.DataMapping,
		DataType:        req.DataType,
		TriggerURL:      req.TriggerURL,
		TriggerSourceID: req.TriggerSourceID,
		TriggerEnabled:  req.TriggerEnabled,
		Enabled:         true,
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
		DataMapping    *json.RawMessage `json:"data_mapping"`
		TriggerEnabled *bool            `json:"trigger_enabled"`
		TriggerURL     *string          `json:"trigger_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DataMapping != nil {
		config.DataMapping = datatypes.JSON(*req.DataMapping)
	}
	if req.TriggerEnabled != nil {
		config.TriggerEnabled = *req.TriggerEnabled
	}
	if req.TriggerURL != nil {
		config.TriggerURL = req.TriggerURL
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
