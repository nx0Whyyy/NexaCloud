package telemetry

import (
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/agent/internal/docker"
	"gorm.io/gorm"
)

type Collector struct {
	db        *gorm.DB
	dockerMgr *docker.Manager
	nc        *nats.Conn
	logger    *slog.Logger
	nodeID    string
	interval  time.Duration
}

func New(db *gorm.DB, dm *docker.Manager, logger *slog.Logger) *Collector {
	return &Collector{
		db:        db,
		dockerMgr: dm,
		logger:    logger,
		nodeID:    "self",
		interval:  10 * time.Second,
	}
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for range ticker.C {
		c.collect()
	}
}

func (c *Collector) collect() {
	_ = time.Now()
}
