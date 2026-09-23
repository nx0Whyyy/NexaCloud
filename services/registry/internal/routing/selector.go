package routing

import (
	"log/slog"
	"sort"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
	"gorm.io/gorm"
)

type Router struct {
	db     *gorm.DB
	logger *slog.Logger
}

func New(db *gorm.DB, logger *slog.Logger) *Router {
	return &Router{db: db, logger: logger}
}

type RouteCandidate struct {
	Instance *model.Instance
	Service  model.RegisteredService
	Score    float64
}

func (r *Router) SelectInstance(serviceName string) *model.RegisteredService {
	var regs []model.RegisteredService
	if err := r.db.Where("service_name = ? AND ready = ?", serviceName, true).Find(&regs).Error; err != nil {
		r.logger.Error("routing query failed", "error", err)
		return nil
	}
	if len(regs) == 0 {
		return nil
	}
	if len(regs) == 1 {
		r.db.First(&model.RegisteredService{}, "instance_id = ?", regs[0].InstanceID)
		return &regs[0]
	}

	candidates := r.scoreInstances(regs)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	r.logger.Info("routing candidates", "service", serviceName,
		"best", candidates[0].Instance.Name, "score", candidates[0].Score)

	return &candidates[0].Service
}

func (r *Router) scoreInstances(regs []model.RegisteredService) []RouteCandidate {
	candidates := make([]RouteCandidate, 0, len(regs))
	for i := range regs {
		var inst model.Instance
		r.db.First(&inst, "id = ?", regs[i].InstanceID)
		score := 100.0
		if inst.Health != nil {
			score -= inst.Health.MSPT * 0.5
			score += inst.Health.TPS * 2
		}
		score -= float64(inst.PlayerCount) * 0.3
		if inst.Pulse == model.PulseCritical {
			score = 0
		}
		candidates = append(candidates, RouteCandidate{
			Instance: &inst,
			Service:  regs[i],
			Score:    score,
		})
	}
	return candidates
}

func (r *Router) Handle(msg *nats.Msg) {
	r.logger.Info("routing request", "subject", msg.Subject)
}
