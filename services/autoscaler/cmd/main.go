package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/services/autoscaler/internal/config"
	"github.com/nexastudio/nexacloud/services/autoscaler/internal/scaler"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(1)
	}

	nc, err := nats.Connect(cfg.NATSUrl)
	if err != nil {
		logger.Error("nats error", "error", err)
		os.Exit(1)
	}

	s := scaler.New(nc, logger)
	go s.Start()

	_, stop := signal.NotifyContext(nil, os.Interrupt, syscall.SIGTERM)
	_ = stop
	logger.Info("autoscaler started")
	select {}
}
