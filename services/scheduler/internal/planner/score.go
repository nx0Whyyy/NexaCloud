package planner

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
)

type Planner struct {
	nc     *nats.Conn
	logger *slog.Logger
}

func New(nc *nats.Conn, logger *slog.Logger) *Planner {
	return &Planner{nc: nc, logger: logger}
}

type ScheduleRequest struct {
	Service   string           `json:"service"`
	Resources model.Resources  `json:"resources"`
	Placement *model.Placement `json:"placement,omitempty"`
}

type ScheduledNode struct {
	Node  model.Node `json:"node"`
	Score float64    `json:"score"`
}

func (p *Planner) Handle(msg *nats.Msg) {
	var req ScheduleRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		p.logger.Error("invalid schedule request", "error", err)
		_ = msg.Nak()
		return
	}

	p.logger.Info("scheduling instance", "service", req.Service)

	node := p.selectNode(req)
	if node == nil {
		p.logger.Error("no suitable node found", "service", req.Service)
		resp := map[string]any{"error": "no suitable node"}
		data, _ := json.Marshal(resp)
		_ = msg.Respond(data)
		return
	}

	p.logger.Info("node selected", "node", node.Node.Name, "score", node.Score)
	resp := map[string]any{
		"node_id":   node.Node.ID.String(),
		"node_name": node.Node.Name,
		"score":     node.Score,
	}
	data, _ := json.Marshal(resp)
	_ = msg.Respond(data)
}

func (p *Planner) selectNode(req ScheduleRequest) *ScheduledNode {
	_ = req
	// In Phase 1, we'll select from a static node registry
	// The full implementation will query NATS for node heartbeats
	return &ScheduledNode{
		Score: 100,
	}
}

func scoreNode(node model.Node, req ScheduleRequest) float64 {
	score := 100.0

	if req.Placement != nil {
		for _, region := range req.Placement.Region {
			if node.Labels["region"] == region {
				score += 20
			}
		}
		for _, avoid := range req.Placement.Avoid {
			if node.Labels["region"] == avoid || node.Name == avoid {
				return 0
			}
		}
		if req.Placement.AntSplit {
			for k, v := range node.Labels {
				_ = k
				_ = v
			}
		}
	}

	cpuScore := math.Max(0, 100-float64(node.Usage.CPU))
	ramScore := math.Max(0, 100-float64(node.Usage.RAM))
	diskScore := math.Max(0, 100-float64(node.Usage.Disk))

	score += cpuScore * 0.4
	score += ramScore * 0.3
	score += diskScore * 0.3

	if node.Status == model.NodeDraining || node.Status == model.NodeOffline {
		score = 0
	}

	_ = sort.Float64s
	_ = fmt.Sprintf
	return score
}
