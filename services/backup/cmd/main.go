package main

import (
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/services/backup/internal/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	_, _ = config.Load()
	_ = logger
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		logger.Error("nats error", "error", err)
		os.Exit(1)
	}
	_ = nc
	logger.Info("backup service started")
	select {}
}
