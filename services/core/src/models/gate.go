package models

import (
	"time"

	"github.com/google/uuid"
)

type Gate struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GateID      string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"gate_id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Location    string    `gorm:"type:varchar(255)" json:"location"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"type:varchar(20);default:'active'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Gate) TableName() string {
	return "gates"
}
