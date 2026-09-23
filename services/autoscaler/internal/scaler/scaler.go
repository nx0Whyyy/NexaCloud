package scaler

import (
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type Scaler struct {
	nc     *nats.Conn
	logger *slog.Logger
}

func New(nc *nats.Conn, logger *slog.Logger) *Scaler {
	return &Scaler{nc: nc, logger: logger}
}

func (s *Scaler) Start() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		s.check()
	}
}

func (s *Scaler) check() {
	s.logger.Debug("autoscaler check")
}
