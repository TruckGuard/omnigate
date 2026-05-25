package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Trigger is one entry in the device's trigger list.
// SourceID identifies the target device that Puller will poll.
// The actual pull URL lives on the target device's TriggerURL field.
type Trigger struct {
	SourceID string `json:"source_id"`
}

type DeviceConfig struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SourceID    string     `gorm:"type:varchar(100);not null" json:"source_id"`
	EventTypeID uuid.UUID  `gorm:"type:uuid;not null" json:"event_type_id"`
	EventType   *EventType `gorm:"foreignKey:EventTypeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"event_type,omitempty"`
	GateID      string     `gorm:"type:varchar(50);not null" json:"gate_id"`

	DataMapping datatypes.JSON `gorm:"type:jsonb;not null" json:"data_mapping"`
	DataType    string         `gorm:"type:varchar(10);not null" json:"data_type"`

	// TriggerURL is THIS device's polling endpoint.
	// Puller calls it when this device is the target of another device's trigger.
	TriggerURL *string `gorm:"type:varchar(500)" json:"trigger_url"`

	// Triggers is the list of target devices this device activates after its own event.
	// Each entry holds the target's source_id; Puller resolves the URL from the target's config.
	Triggers       datatypes.JSON `gorm:"type:jsonb" json:"triggers"`
	TriggerEnabled bool           `gorm:"default:false" json:"trigger_enabled"`

	// ImageFields lists data_mapping field names whose values are base64-encoded images.
	// The Adapter decodes them, uploads to S3, and replaces the value with the object key.
	ImageFields datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"image_fields"`

	// AwaitSourceIDs is the list of source_ids this device expects events from.
	// After this device's event is processed, a tx_await key is registered for each entry
	// so that the awaited device's next event is pulled into the same transaction.
	AwaitSourceIDs datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"await_source_ids"`
	// AwaitTTLSeconds is the expiry for each tx_await key. 0 falls back to the gate TTL.
	AwaitTTLSeconds int `gorm:"default:0" json:"await_ttl_seconds"`

	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DeviceConfig) TableName() string {
	return "device_configs"
}
