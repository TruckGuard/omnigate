package models

import (
	"encoding/json"
	"time"
)

type IngestEvent struct {
	SourceID      string    `json:"source_id"`
	SourceName    string    `json:"source_name"`
	GateID        string    `json:"gate_id"`
	Payload       string    `json:"payload"`
	RawStorageKey string    `json:"raw_storage_key"`
	ImageKeys     []string  `json:"image_keys"`
	TransactionID *string   `json:"transaction_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

func (e *IngestEvent) ToJSON() string {
	b, _ := json.Marshal(e)
	return string(b)
}

type CoreDeviceConfig struct {
	GateID          string  `json:"gate_id"`
	TriggerEnabled  bool    `json:"trigger_enabled"`
	TriggerURL      *string `json:"trigger_url"`
	TriggerSourceID *string `json:"trigger_source_id"`
}


