package model

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	Slug      string    `json:"slug" gorm:"uniqueIndex;size:80;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OrganizationMember struct {
	OrganizationID uuid.UUID `json:"organization_id" gorm:"type:uuid;primaryKey"`
	UserID         uuid.UUID `json:"user_id" gorm:"type:uuid;primaryKey"`
	Role           string    `json:"role" gorm:"size:32;not null"`
	CreatedAt      time.Time `json:"created_at"`
}

type Plan struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Code         string    `json:"code" gorm:"uniqueIndex;size:32;not null"`
	Name         string    `json:"name" gorm:"size:64;not null"`
	PriceMonthly int64     `json:"price_monthly_cents"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PlanEntitlement struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	PlanID    uuid.UUID `json:"plan_id" gorm:"type:uuid;uniqueIndex:idx_plan_entitlement;not null"`
	Key       string    `json:"key" gorm:"uniqueIndex:idx_plan_entitlement;size:80;not null"`
	Value     string    `json:"value" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Subscription struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	OrganizationID    uuid.UUID  `json:"organization_id" gorm:"type:uuid;uniqueIndex;not null"`
	PlanID            uuid.UUID  `json:"plan_id" gorm:"type:uuid;not null"`
	Status            string     `json:"status" gorm:"size:20;not null"`
	EntitlementData   string     `json:"-" gorm:"type:jsonb;not null"`
	CurrentPeriodEnds *time.Time `json:"current_period_ends,omitempty"`
	GraceEndsAt       *time.Time `json:"grace_ends_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type License struct {
	ID             uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID  `json:"organization_id" gorm:"type:uuid;index;not null"`
	KeyPrefix      string     `json:"key_prefix" gorm:"size:12;not null"`
	KeyHash        string     `json:"-" gorm:"uniqueIndex;size:64;not null"`
	Status         string     `json:"status" gorm:"size:20;not null"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
}

type Network struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `json:"organization_id" gorm:"type:uuid;index;not null"`
	Name           string    `json:"name" gorm:"size:100;not null"`
	Status         string    `json:"status" gorm:"size:20;not null"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type InstanceSlot struct {
	InstanceID     uuid.UUID `json:"instance_id" gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `json:"organization_id" gorm:"type:uuid;index;not null"`
	ReservedAt     time.Time `json:"reserved_at"`
}

type NodeCredential struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	NodeID      uuid.UUID  `json:"node_id" gorm:"type:uuid;index;not null"`
	PublicKey   string     `json:"public_key" gorm:"type:text;not null"`
	Fingerprint string     `json:"fingerprint" gorm:"uniqueIndex;size:64;not null"`
	SecretHash  string     `json:"-" gorm:"size:64;index"`
	Status      string     `json:"status" gorm:"size:20;not null"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

type DeviceEnrollment struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	CodeHash        string     `json:"-" gorm:"uniqueIndex;size:64;not null"`
	DeviceTokenHash string     `json:"-" gorm:"uniqueIndex;size:64;not null"`
	NodeName        string     `json:"node_name" gorm:"size:100;not null"`
	PublicKey       string     `json:"-" gorm:"type:text;not null"`
	Resources       Resources  `json:"resources" gorm:"type:jsonb"`
	Status          string     `json:"status" gorm:"size:20;not null"`
	NodeID          *uuid.UUID `json:"node_id,omitempty" gorm:"type:uuid"`
	CreatedAt       time.Time  `json:"created_at"`
	ExpiresAt       time.Time  `json:"expires_at" gorm:"index;not null"`
	ApprovedAt      *time.Time `json:"approved_at,omitempty"`
	ClaimedAt       *time.Time `json:"claimed_at,omitempty"`
}
