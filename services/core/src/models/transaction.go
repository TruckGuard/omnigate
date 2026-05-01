package models

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Code         string     `gorm:"type:varchar(50);unique;not null" json:"code"`
	GateID       string     `gorm:"type:varchar(50);not null" json:"gate_id"`
	Status       string     `gorm:"type:varchar(20);default:'active'" json:"status"`
	
	Events       []Event    `gorm:"foreignKey:TransactionID" json:"events,omitempty"`
	
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

func (Transaction) TableName() string {
	return "transactions"
}
