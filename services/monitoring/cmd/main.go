package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nexastudio/nexacloud/services/monitoring/internal/config"
	"github.com/nexastudio/nexacloud/services/monitoring/internal/server"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}

	app, err := server.New(cfg, db, logger)
	if err != nil {
		logger.Error("monitoring initialization failed", "error", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.Run(ctx)

	httpServer := &http.Server{Addr: fmt.Sprintf(":%d", cfg.APIPort), Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("monitoring API started", "port", cfg.APIPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("monitoring API failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	_ = httpServer.Shutdown(shutdown)
}
