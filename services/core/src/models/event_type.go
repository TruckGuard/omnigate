package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type EventType struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code          string         `gorm:"type:varchar(50);unique;not null" json:"code"`
	Name          string         `gorm:"type:varchar(200);not null" json:"name"`
	Description   string         `gorm:"type:text" json:"description"`
	Fields        datatypes.JSON `gorm:"type:jsonb;not null" json:"fields"`
	SearchableKey string         `gorm:"type:varchar(50);default:''" json:"searchable_key"`
	CreatedAt     time.Time      `json:"created_at"`
}

func (EventType) TableName() string {
	return "event_types"
}
