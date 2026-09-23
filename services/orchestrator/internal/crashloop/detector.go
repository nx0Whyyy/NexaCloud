package crashloop

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

type Detector struct {
	db     *gorm.DB
	logger *slog.Logger
}

type crashRecord struct {
	InstanceID string
	Count      int
	Since      time.Time
	Reason     string
}

var crashes = make(map[string]crashRecord)

func New(db *gorm.DB, logger *slog.Logger) *Detector {
	return &Detector{db: db, logger: logger}
}

func (d *Detector) Check(ctx context.Context) error {
	var instances []model.Instance
	if err := d.db.WithContext(ctx).Where("status = ?", model.InstanceCrashed).
		Find(&instances).Error; err != nil {
		return fmt.Errorf("query crashed instances: %w", err)
	}

	for _, inst := range instances {
		rec, exists := crashes[inst.ID.String()]
		if !exists {
			reason := ""
			if inst.Health != nil {
				reason = inst.Health.LastCrashReason
			}
			rec = crashRecord{
				InstanceID: inst.ID.String(),
				Count:      0,
				Since:      time.Now(),
				Reason:     reason,
			}
		}

		rec.Count++
		if rec.Count >= 3 && time.Since(rec.Since) < 94*time.Second {
			d.logger.Warn("crash loop detected, quarantining",
				"instance", inst.Name, "crashes", rec.Count, "reason", rec.Reason)

			inst.Status = model.InstanceQuarantined
			inst.Pulse = model.PulseCritical
			d.db.WithContext(ctx).Save(&inst)

			_ = d.db.WithContext(ctx).Create(&model.TimelineEvent{
				ID:           model.NewID(),
				Actor:        "system",
				Action:       "quarantine",
				ResourceType: "instance",
				ResourceID:   inst.ID.String(),
				Details:      fmt.Sprintf("Crash loop detected: %d crashes in %v. Reason: %s", rec.Count, time.Since(rec.Since), rec.Reason),
				CreatedAt:    model.Now(),
			}).Error

			delete(crashes, inst.ID.String())
		} else {
			crashes[inst.ID.String()] = rec
		}
	}
	return nil
}
