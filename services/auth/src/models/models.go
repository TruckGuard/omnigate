package models

import "time"

type Permission struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description"`
	Module      string `json:"module"`
}

type Role struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	Name        string       `gorm:"unique;not null" json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions"`
}

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"unique;not null" json:"username"`
	PasswordHash string     `gorm:"not null" json:"-"`
	RoleID       uint       `json:"role_id"`
	Role         Role       `gorm:"foreignKey:RoleID" json:"role"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login"`
}

type APIKey struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	KeyHash     string       `gorm:"unique;index;not null" json:"-"`
	OwnerName   string       `json:"owner_name"`
	IsActive    bool         `gorm:"default:true" json:"is_active"`
	GateID      string       `gorm:"type:varchar(50)" json:"gate_id"`
	Permissions []Permission `gorm:"many2many:apikey_permissions;" json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	// Digest Auth for ITSAPI cameras. NULL = digest disabled for this key.
	// DigestHA1 stores MD5(username:realm:password) — plaintext password is never persisted.
	DigestUsername *string `gorm:"type:varchar(255);uniqueIndex" json:"digest_username,omitempty"`
	DigestHA1      *string `gorm:"type:varchar(32)" json:"-"`
}

type SourceMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	GateID      string   `json:"gate_id"`
	Permissions []string `json:"permissions"`
}

type PolicyRule struct {
	ID                 uint   `gorm:"primaryKey" json:"id"`
	Method             string `gorm:"not null" json:"method"`              // GET, POST, PUT, DELETE, *
	PathPattern        string `gorm:"not null;index" json:"path_pattern"`  // Regex: ^/api/users.*
	RequiredPermission string `gorm:"not null" json:"required_permission"` // e.g., read:users
	Description        string `json:"description"`
}

type PermissionHierarchy struct {
	ParentID string     `gorm:"primaryKey;column:parent_id" json:"parent_id"`
	Parent   Permission `gorm:"foreignKey:ParentID" json:"-"`
	ChildID  string     `gorm:"primaryKey;column:child_id" json:"child_id"`
	Child    Permission `gorm:"foreignKey:ChildID" json:"-"`
}
