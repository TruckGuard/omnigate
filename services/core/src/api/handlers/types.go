package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
	"github.com/omnigate/services/core/src/repository"
	"gorm.io/datatypes"
)

func HandleListEventTypes(c *gin.Context) {
	types := repository.ListEventTypes()
	c.JSON(http.StatusOK, types)
}

func HandleGetEventType(c *gin.Context) {
	id := c.Param("id")
	typeID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type ID"})
		return
	}

	eventType := repository.GetEventType(typeID)
	if eventType == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event type not found"})
		return
	}

	c.JSON(http.StatusOK, eventType)
}

func HandleCreateEventType(c *gin.Context) {
	var req struct {
		Code        string         `json:"code" binding:"required"`
		Name        string         `json:"name" binding:"required"`
		Description string         `json:"description"`
		Fields      datatypes.JSON `json:"fields" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eventType := &models.EventType{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Fields:      req.Fields,
	}

	savedType := repository.CreateEventType(eventType)
	c.JSON(http.StatusCreated, savedType)
}
