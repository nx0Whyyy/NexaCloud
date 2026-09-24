package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	Username        string     `json:"username" gorm:"uniqueIndex;size:32;not null"`
	Email           string     `json:"email" gorm:"uniqueIndex;size:255;not null"`
	PasswordHash    string     `json:"-" gorm:"size:255;not null"`
	Role            string     `json:"role" gorm:"size:20;not null;default:user"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	DisabledAt      *time.Time `json:"disabled_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type EmailVerification struct {
	TokenHash string    `json:"-" gorm:"primaryKey;size:64"`
	UserID    uuid.UUID `json:"-" gorm:"type:uuid;index;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index;not null"`
	CreatedAt time.Time `json:"created_at"`
}

type UserSession struct {
	TokenHash string    `json:"-" gorm:"primaryKey;size:64"`
	UserID    uuid.UUID `json:"-" gorm:"type:uuid;index;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index;not null"`
	CreatedAt time.Time `json:"created_at"`
}
