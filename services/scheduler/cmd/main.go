package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/services/scheduler/internal/config"
	"github.com/nexastudio/nexacloud/services/scheduler/internal/planner"
)

var (
	version = "dev"
	commit  = "unknown"
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
		logger.Error("nats connect error", "error", err)
		os.Exit(1)
	}

	p := planner.New(nc, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("scheduler starting", "version", version)

	sub, err := nc.QueueSubscribe("schedule.instance", "scheduler", p.Handle)
	if err != nil {
		logger.Error("subscribe error", "error", err)
		os.Exit(1)
	}
	defer sub.Drain()

	<-ctx.Done()
	logger.Info("scheduler shutting down")
}
