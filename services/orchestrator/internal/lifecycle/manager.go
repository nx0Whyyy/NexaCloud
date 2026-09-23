package lifecycle

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

type Manager struct {
	db     *gorm.DB
	nc     *nats.Conn
	logger *slog.Logger
}

func New(db *gorm.DB, nc *nats.Conn, logger *slog.Logger) *Manager {
	return &Manager{db: db, nc: nc, logger: logger}
}

func (m *Manager) Reconcile(ctx context.Context) error {
	var services []model.Service
	if err := m.db.WithContext(ctx).Find(&services).Error; err != nil {
		return fmt.Errorf("query services: %w", err)
	}

	for _, svc := range services {
		if err := m.reconcileService(ctx, &svc); err != nil {
			m.logger.Error("reconcile service failed", "service", svc.Name, "error", err)
		}
	}
	return nil
}

func (m *Manager) reconcileService(ctx context.Context, svc *model.Service) error {
	var instances []model.Instance
	if err := m.db.WithContext(ctx).Where("service_name = ?", svc.Name).Find(&instances).Error; err != nil {
		return err
	}

	ready := 0
	for _, inst := range instances {
		if inst.Status == model.InstanceActive || inst.Status == model.InstanceReady {
			ready++
		}
	}

	desired := svc.Autoscaling.Minimum
	if desired == 0 {
		desired = 1
	}

	if ready < desired {
		m.logger.Info("creating instance", "service", svc.Name, "need", desired-ready, "have", ready)
		return m.createInstance(ctx, svc)
	}

	if ready > desired && len(instances) > desired {
		m.logger.Info("draining excess instance", "service", svc.Name)
		return m.drainExcess(ctx, svc, instances, desired)
	}

	return nil
}

func (m *Manager) createInstance(ctx context.Context, svc *model.Service) error {
	instName := fmt.Sprintf("%s-%d", svc.Name, m.countByService(ctx, svc.Name)+1)

	inst := &model.Instance{
		ID:          model.NewID(),
		Name:        instName,
		ServiceName: svc.Name,
		Status:      model.InstanceCreating,
		Pulse:       model.PulseOffline,
		Resources:   svc.Resources,
		Metadata: model.InstanceMeta{
			Software: svc.Software,
			Java:     svc.Software.Java,
		},
		PlayerCount: 0,
		CreatedAt:   model.Now(),
		UpdatedAt:   model.Now(),
	}

	if err := m.db.WithContext(ctx).Create(inst).Error; err != nil {
		return fmt.Errorf("create instance: %w", err)
	}

	cmd := &model.CommandRequest{
		ID:      model.NewID().String(),
		Command: "CREATE_INSTANCE",
		InstanceID: inst.ID.String(),
		Params: map[string]string{
			"service":  svc.Name,
			"software": string(svc.Software.Type),
			"version":  svc.Software.Version,
			"memory":   svc.Resources.Memory,
		},
	}

	if err := m.nc.Publish("commands.orchestrator", mustMarshal(cmd)); err != nil {
		return fmt.Errorf("publish command: %w", err)
	}

	return m.createTimeline(ctx, "system", "create_instance", "instance", inst.ID.String(),
		fmt.Sprintf("Creating %s for service %s", instName, svc.Name))
}

func (m *Manager) drainExcess(ctx context.Context, svc *model.Service, instances []model.Instance, desired int) error {
	excess := len(instances) - desired
	count := 0
	for i := range instances {
		if count >= excess {
			break
		}
		if instances[i].Status != model.InstanceActive && instances[i].Status != model.InstanceReady {
			continue
		}
		instances[i].Status = model.InstanceDraining
		m.db.WithContext(ctx).Save(&instances[i])
		count++
		m.logger.Info("draining instance", "instance", instances[i].Name)
	}
	return nil
}

func (m *Manager) countByService(ctx context.Context, service string) int {
	var count int64
	_ = m.db.WithContext(ctx).Model(&model.Instance{}).Where("service_name = ?", service).Count(&count)
	return int(count)
}

func (m *Manager) createTimeline(ctx context.Context, actor, action, resourceType, resourceID, details string) error {
	return m.db.WithContext(ctx).Create(&model.TimelineEvent{
		ID:          model.NewID(),
		Actor:       actor,
		Action:      action,
		ResourceType: resourceType,
		ResourceID:  resourceID,
		Details:     details,
		CreatedAt:   model.Now(),
	}).Error
}

func mustMarshal(v any) []byte {
	b, _ := jsonMarshal(v)
	return b
}

func jsonMarshal(v any) ([]byte, error) {
	return []byte(model.JSON(v)), nil
}
