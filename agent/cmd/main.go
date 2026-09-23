package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/agent/internal/command"
	"github.com/nexastudio/nexacloud/agent/internal/config"
	"github.com/nexastudio/nexacloud/agent/internal/docker"
	"github.com/nexastudio/nexacloud/agent/internal/telemetry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("db error", "error", err)
		os.Exit(1)
	}

	nc, err := nats.Connect(cfg.NATSUrl)
	if err != nil {
		logger.Error("nats error", "error", err)
		os.Exit(1)
	}

	dockerMgr := docker.New(logger)
	cmdHandler := command.New(db, dockerMgr, logger)
	tl := telemetry.New(db, dockerMgr, logger)

	sub, err := nc.QueueSubscribe("commands.agent."+cfg.NodeID+".>", "agent", cmdHandler.Handle)
	if err != nil {
		logger.Error("subscribe error", "error", err)
		os.Exit(1)
	}
	defer sub.Drain()

	go tl.Start()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("nexa-agent started", "version", version, "node_id", cfg.NodeID)
	<-ctx.Done()
	logger.Info("agent shutting down")
}
