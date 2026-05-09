package repository

import (
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func CreateEventType(t *models.EventType) *models.EventType {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	DB.Create(t)
	return t
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

func UpdateEventType(id uuid.UUID, updates map[string]interface{}) *models.EventType {
	var eventType models.EventType
	if err := DB.First(&eventType, id).Error; err != nil {
		return nil
	}
	DB.Model(&eventType).Updates(updates)
	return &eventType
}
