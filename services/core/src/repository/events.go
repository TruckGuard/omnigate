package repository

import (
	"github.com/google/uuid"
	"github.com/omnigate/services/core/src/models"
)

func CreateEvent(event *models.Event) *models.Event {
	DB.Create(event)
	return event
}

func ListEvents(transactionID, gateID, sourceID string) []models.Event {
	var events []models.Event
	query := DB.Model(&models.Event{})
	if transactionID != "" {
		query = query.Where("transaction_id = ?", transactionID)
	}
	if gateID != "" {
		query = query.Where("gate_id = ?", gateID)
	}
	if sourceID != "" {
		query = query.Where("source_id = ?", sourceID)
	}
	query.Find(&events)
	return events
}

func GetEvent(id uuid.UUID) *models.Event {
	var event models.Event
	if err := DB.First(&event, id).Error; err != nil {
		return nil
	}
	return &event
}

func DeleteEvent(id uuid.UUID) error {
	return DB.Delete(&models.Event{}, id).Error
}

func GetLatestEventForSource(sourceID string) *models.Event {
	var event models.Event
	if err := DB.Where("source_id = ?", sourceID).Order("created_at DESC").First(&event).Error; err != nil {
		return nil
	}
	return &event
}
