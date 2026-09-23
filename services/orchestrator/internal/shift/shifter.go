package shift

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

type Shifter struct {
	db     *gorm.DB
	nc     *nats.Conn
	logger *slog.Logger
}

func New(db *gorm.DB, nc *nats.Conn, logger *slog.Logger) *Shifter {
	return &Shifter{db: db, nc: nc, logger: logger}
}

func (s *Shifter) Reconcile(ctx context.Context) error {
	var instances []model.Instance
	if err := s.db.WithContext(ctx).Where("status = ? OR status = ?", model.InstanceActive, model.InstanceReady).
		Find(&instances).Error; err != nil {
		return fmt.Errorf("query instances: %w", err)
	}
	for range instances {
	}
	return nil
}
