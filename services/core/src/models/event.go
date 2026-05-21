package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Event struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TransactionID *uuid.UUID     `gorm:"type:uuid" json:"transaction_id"`

	EventTypeID uuid.UUID  `gorm:"type:uuid;not null" json:"event_type_id"`
	EventType   *EventType `gorm:"foreignKey:EventTypeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"event_type,omitempty"`

	GateID   string `gorm:"type:varchar(50);not null" json:"gate_id"`
	SourceID string `gorm:"type:varchar(100);not null" json:"source_id"`

	Data       datatypes.JSON `gorm:"type:jsonb;not null" json:"data"`
	RawDataKey string         `gorm:"type:varchar(500)" json:"raw_data_key"`
	ImageKeys  datatypes.JSON `gorm:"type:text[]" json:"image_keys"`

	// TypeCode — денормалізована копія EventType.Code.
	// Зберігається в БД, щоб уникати JOIN при пошуку.
	// Заповнюється автоматично хуком BeforeSave.
	TypeCode string `gorm:"type:varchar(50);column:type_code;index" json:"type_code"`

	// SearchableValue — матеріалізоване поле для нечіткого пошуку.
	// Заповнюється BeforeSave з data[EventType.SearchableKey] (нормалізовано: без пробілів, верхній регістр).
	// На це поле будується GIN-індекс pg_trgm (дивись MigrateDB).
	SearchableValue string `gorm:"type:text;column:searchable_value" json:"searchable_value"`

	CreatedAt time.Time `json:"created_at"`
}

func (Event) TableName() string {
	return "events"
}

// BeforeSave — GORM-хук, що заповнює TypeCode та SearchableValue перед записом у БД.
// Спрацьовує при Create та Save (але не при часткових Update/Updates).
func (e *Event) BeforeSave(tx *gorm.DB) error {
	var et EventType
	if e.EventType != nil {
		et = *e.EventType
	} else if e.EventTypeID != uuid.Nil {
		// Session{NewDB: true} уникає конфліктів з поточним statement у хуку.
		tx.Session(&gorm.Session{NewDB: true}).
			Select("code", "searchable_key").
			First(&et, e.EventTypeID)
	}

	e.TypeCode = et.Code

	if et.SearchableKey == "" {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return nil
	}
	rawValue, ok := payload[et.SearchableKey]
	if !ok {
		return nil
	}
	strValue, ok := rawValue.(string)
	if !ok || strValue == "" {
		return nil
	}

	e.SearchableValue = strings.ToUpper(strings.ReplaceAll(strValue, " ", ""))
	return nil
}
