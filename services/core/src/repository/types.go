package repository

import (
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func CreateEventType(eventType *models.EventType) *models.EventType {
	DB.Create(eventType)
	return eventType
}

func ListEventTypes() []models.EventType {
	var types []models.EventType
	DB.Find(&types)
	return types
}

func GetEventType(id uuid.UUID) *models.EventType {
	var eventType models.EventType
	if err := DB.First(&eventType, id).Error; err != nil {
		return nil
	}
	return &eventType
}
