package models

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code   string    `gorm:"type:varchar(50);unique;not null" json:"code"`
	GateID string    `gorm:"type:varchar(50);not null" json:"gate_id"`
	Note   string    `gorm:"type:text" json:"note"`

	Events []Event `gorm:"foreignKey:TransactionID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"events,omitempty"`

	// IsOpen is populated at query time by checking Valkey — not stored in DB.
	IsOpen bool `gorm:"-" json:"is_open"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Transaction) TableName() string {
	return "transactions"
}
