package access

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const FreePlanCode = "free"

var ErrQuotaExceeded = errors.New("active instance quota exceeded")

type Entitlements struct {
	Features map[string]bool `json:"features"`
	Limits   map[string]int  `json:"limits"`
}

type Service struct{ db *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) Migrate() error {
	if err := s.db.AutoMigrate(
		&model.Organization{}, &model.OrganizationMember{}, &model.Plan{},
		&model.PlanEntitlement{}, &model.Subscription{}, &model.License{},
		&model.Network{}, &model.InstanceSlot{}, &model.Node{}, &model.Service{}, &model.Instance{},
		&model.NodeCredential{}, &model.DeviceEnrollment{},
		&model.AuditEntry{},
	); err != nil {
		return err
	}
	if err := s.seedFreePlan(); err != nil {
		return err
	}
	return s.backfillUsers()
}

func (s *Service) backfillUsers() error {
	var users []model.User
	if err := s.db.Where("NOT EXISTS (?)", s.db.Model(&model.OrganizationMember{}).Select("1").Where("organization_members.user_id = users.id")).Find(&users).Error; err != nil {
		return err
	}
	for i := range users {
		if err := s.db.Transaction(func(tx *gorm.DB) error { return s.ProvisionUser(tx, &users[i]) }); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) seedFreePlan() error {
	now := model.Now()
	plan := model.Plan{ID: model.NewID(), Code: FreePlanCode, Name: "Free", Active: true, CreatedAt: now, UpdatedAt: now}
	if err := s.db.Where("code = ?", FreePlanCode).FirstOrCreate(&plan).Error; err != nil {
		return err
	}
	values := map[string]string{
		"max_networks": "1", "max_nodes": "1", "max_active_instances": "3",
		"metric_retention_days": "1", "feature_manual_backups": "true",
		"feature_basic_monitoring": "true", "feature_autoscaling": "false",
		"feature_canary": "false", "feature_nexa_shift": "false",
	}
	for key, value := range values {
		entry := model.PlanEntitlement{ID: model.NewID(), PlanID: plan.ID, Key: key, Value: value, CreatedAt: now, UpdatedAt: now}
		if err := s.db.Where("plan_id = ? AND key = ?", plan.ID, key).Assign(map[string]any{"value": value, "updated_at": now}).FirstOrCreate(&entry).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ProvisionUser(tx *gorm.DB, user *model.User) error {
	var plan model.Plan
	if err := tx.Where("code = ? AND active = ?", FreePlanCode, true).First(&plan).Error; err != nil {
		return err
	}
	entitlements, err := loadPlanEntitlements(tx, plan.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(entitlements)
	if err != nil {
		return err
	}
	now := model.Now()
	org := model.Organization{ID: model.NewID(), Name: user.Username, Slug: fmt.Sprintf("%s-%s", strings.ToLower(user.Username), user.ID.String()[:8]), CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&org).Error; err != nil {
		return err
	}
	if err := tx.Create(&model.OrganizationMember{OrganizationID: org.ID, UserID: user.ID, Role: "owner", CreatedAt: now}).Error; err != nil {
		return err
	}
	return tx.Create(&model.Subscription{ID: model.NewID(), OrganizationID: org.ID, PlanID: plan.ID, Status: "ACTIVE", EntitlementData: string(payload), CreatedAt: now, UpdatedAt: now}).Error
}

func loadPlanEntitlements(db *gorm.DB, planID uuid.UUID) (Entitlements, error) {
	var rows []model.PlanEntitlement
	if err := db.Where("plan_id = ?", planID).Find(&rows).Error; err != nil {
		return Entitlements{}, err
	}
	result := Entitlements{Features: map[string]bool{}, Limits: map[string]int{}}
	for _, row := range rows {
		if strings.HasPrefix(row.Key, "feature_") {
			result.Features[strings.TrimPrefix(row.Key, "feature_")] = row.Value == "true"
			continue
		}
		var value int
		if _, err := fmt.Sscanf(row.Value, "%d", &value); err == nil {
			result.Limits[row.Key] = value
		}
	}
	return result, nil
}

func (s *Service) EntitlementsFor(orgID uuid.UUID) (Entitlements, model.Subscription, error) {
	return entitlementsFor(s.db, orgID)
}

func entitlementsFor(db *gorm.DB, orgID uuid.UUID) (Entitlements, model.Subscription, error) {
	var subscription model.Subscription
	err := db.Where("organization_id = ?", orgID).First(&subscription).Error
	if err != nil {
		return Entitlements{}, subscription, err
	}
	var entitlements Entitlements
	err = json.Unmarshal([]byte(subscription.EntitlementData), &entitlements)
	return entitlements, subscription, err
}

func (s *Service) ReserveInstanceSlot(orgID, instanceID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var subscription model.Subscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", orgID).First(&subscription).Error; err != nil {
			return err
		}
		if subscription.Status != "ACTIVE" && subscription.Status != "TRIAL" && subscription.Status != "GRACE" && subscription.Status != "PAST_DUE" {
			return fmt.Errorf("subscription is %s", strings.ToLower(subscription.Status))
		}
		var entitlements Entitlements
		if err := json.Unmarshal([]byte(subscription.EntitlementData), &entitlements); err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&model.InstanceSlot{}).Where("instance_id = ?", instanceID).Count(&existing).Error; err != nil || existing > 0 {
			return err
		}
		var count int64
		if err := tx.Model(&model.InstanceSlot{}).Where("organization_id = ?", orgID).Count(&count).Error; err != nil {
			return err
		}
		if int(count) >= entitlements.Limits["max_active_instances"] {
			return ErrQuotaExceeded
		}
		return tx.Create(&model.InstanceSlot{InstanceID: instanceID, OrganizationID: orgID, ReservedAt: model.Now()}).Error
	})
}

func (s *Service) ReleaseInstanceSlot(instanceID uuid.UUID) error {
	return s.db.Delete(&model.InstanceSlot{}, "instance_id = ?", instanceID).Error
}

func NewLicenseKey() (string, string, error) {
	raw := make([]byte, 9)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	encoded := strings.ToUpper(hex.EncodeToString(raw))
	key := "NX-" + encoded[:4] + "-" + encoded[4:8] + "-" + encoded[8:12] + "-" + encoded[12:16]
	sum := sha256.Sum256([]byte(key))
	return key, hex.EncodeToString(sum[:]), nil
}

func LicenseUsable(license model.License, now time.Time) bool {
	return license.Status == "ACTIVE" && license.RevokedAt == nil && (license.ExpiresAt == nil || license.ExpiresAt.After(now))
}
