package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type DeviceConfig struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SourceID       string         `gorm:"type:varchar(100);not null" json:"source_id"`
	EventTypeID    uuid.UUID      `gorm:"type:uuid;not null" json:"event_type_id"`
	EventType      *EventType     `gorm:"foreignKey:EventTypeID" json:"event_type,omitempty"`
	GateID         string         `gorm:"type:varchar(50);not null" json:"gate_id"`
	
	DataMapping    datatypes.JSON `gorm:"type:jsonb;not null" json:"data_mapping"`
	DataType       string         `gorm:"type:varchar(10);not null" json:"data_type"`
	
	TriggerURL      *string        `gorm:"type:varchar(500)" json:"trigger_url"`
	TriggerSourceID *string        `gorm:"type:varchar(100)" json:"trigger_source_id"`

	TriggerEnabled bool           `gorm:"default:false" json:"trigger_enabled"`
	Enabled        bool           `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (DeviceConfig) TableName() string {
	return "device_configs"
}
