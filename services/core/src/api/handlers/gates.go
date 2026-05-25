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

func HandleListGates(c *gin.Context) {
	c.JSON(http.StatusOK, repository.ListGates())
}

func HandleGetGate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gate ID"})
		return
	}
	gate := repository.GetGate(id)
	if gate == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}
	c.JSON(http.StatusOK, gate)
}

func HandleCreateGate(c *gin.Context) {
	var req struct {
		GateID      string           `json:"gate_id" binding:"required"`
		Name        string           `json:"name" binding:"required"`
		Location    string           `json:"location"`
		Description string           `json:"description"`
		Settings    *json.RawMessage `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	g := &models.Gate{
		GateID:      req.GateID,
		Name:        req.Name,
		Location:    req.Location,
		Description: req.Description,
		Status:      "active",
	}
	if req.Settings != nil {
		g.Settings = datatypes.JSON(*req.Settings)
	}
	gate := repository.CreateGate(g)
	c.JSON(http.StatusCreated, gate)
}

func HandleUpdateGate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gate ID"})
		return
	}
	gate := repository.GetGate(id)
	if gate == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}

	var req struct {
		Name        string           `json:"name"`
		Location    string           `json:"location"`
		Description string           `json:"description"`
		Status      string           `json:"status"`
		Settings    *json.RawMessage `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		gate.Name = req.Name
	}
	if req.Location != "" {
		gate.Location = req.Location
	}
	if req.Description != "" {
		gate.Description = req.Description
	}
	if req.Status != "" {
		gate.Status = req.Status
	}
	if req.Settings != nil {
		gate.Settings = datatypes.JSON(*req.Settings)
	}

	if err := repository.UpdateGate(gate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gate)
}

func HandleDeleteGate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gate ID"})
		return
	}
	if err := repository.DeleteGate(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Gate deleted"})
}

func HandleUpdateGateSettings(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gate ID"})
		return
	}
	gate := repository.GetGate(id)
	if gate == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	gate.Settings = datatypes.JSON(raw)
	if err := repository.UpdateGate(gate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gate)
}

func HandleGetGateStats(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gate ID"})
		return
	}
	gate := repository.GetGate(id)
	if gate == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gate not found"})
		return
	}
	c.JSON(http.StatusOK, repository.GetGateStats(gate.GateID))
}

