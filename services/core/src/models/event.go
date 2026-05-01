package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Event struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TransactionID *uuid.UUID     `gorm:"type:uuid" json:"transaction_id"`
	
	EventTypeID   uuid.UUID      `gorm:"type:uuid;not null" json:"event_type_id"`
	EventType     *EventType     `gorm:"foreignKey:EventTypeID" json:"event_type,omitempty"`
	
	GateID        string         `gorm:"type:varchar(50);not null" json:"gate_id"`
	SourceID      string         `gorm:"type:varchar(100);not null" json:"source_id"`
	
	Data          datatypes.JSON `gorm:"type:jsonb;not null" json:"data"`
	RawDataKey    string         `gorm:"type:varchar(500)" json:"raw_data_key"`
	ImageKeys     datatypes.JSON `gorm:"type:text[]" json:"image_keys"`
	
	CreatedAt     time.Time      `json:"created_at"`
}

func (Event) TableName() string {
	return "events"
}
