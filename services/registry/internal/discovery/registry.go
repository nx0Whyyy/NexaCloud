package discovery

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

type Registry struct {
	db     *gorm.DB
	nc     *nats.Conn
	logger *slog.Logger
}

func New(db *gorm.DB, nc *nats.Conn, logger *slog.Logger) *Registry {
	return &Registry{db: db, nc: nc, logger: logger}
}

func (r *Registry) Handle(msg *nats.Msg) {
	subject := msg.Subject
	r.logger.Info("registry message", "subject", subject)

	var pkt model.TelemetryPacket
	if err := json.Unmarshal(msg.Data, &pkt); err != nil {
		r.logger.Error("invalid telemetry", "error", err)
		return
	}

	if pkt.InstanceID == nil {
		return
	}

	instID := *pkt.InstanceID

	var inst model.Instance
	if err := r.db.First(&inst, "id = ?", instID).Error; err != nil {
		r.logger.Error("instance not found", "id", instID, "error", err)
		return
	}

	reg := model.RegisteredService{
		InstanceID:   inst.ID,
		ServiceName:  inst.ServiceName,
		Address:      pkt.Source,
		Port:         0,
		Ready:        true,
		RegisteredAt: model.Now(),
	}

	_ = r.db.Save(&reg)
	r.logger.Info("instance registered", "name", inst.Name, "service", inst.ServiceName)
}

func (r *Registry) Unregister(instanceID string) error {
	r.db.Delete(&model.RegisteredService{}, "instance_id = ?", instanceID)
	return nil
}

func (r *Registry) GetService(serviceName string) (*model.RegisteredService, error) {
	var reg model.RegisteredService
	err := r.db.Where("service_name = ? AND ready = ?", serviceName, true).
		Order("registered_at DESC").First(&reg).Error
	if err != nil {
		return nil, fmt.Errorf("registry query: %w", err)
	}
	return &reg, nil
}
