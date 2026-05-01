package models

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile stores contact information for auth service users.
// AuthID references the user ID in the auth service (omnigate_auth.users).
type UserProfile struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AuthID    uint      `gorm:"uniqueIndex;not null" json:"auth_id"`
	FirstName string    `gorm:"type:varchar(100)" json:"first_name"`
	LastName  string    `gorm:"type:varchar(100)" json:"last_name"`
	Phone     string    `gorm:"type:varchar(50)" json:"phone"`
	GateID    string    `gorm:"type:varchar(50)" json:"gate_id"`
	Notes     string    `gorm:"type:text" json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserProfile) TableName() string {
	return "user_profiles"
}
