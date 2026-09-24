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

	"github.com/nexastudio/nexacloud/services/orchestrator/internal/config"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/reconciler"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	orc, err := reconciler.New(ctx, logger, cfg)
	if err != nil {
		logger.Error("failed to initialize orchestrator", "error", err)
		os.Exit(1)
	}

	go func() {
		if err := orc.Run(ctx); err != nil {
			logger.Error("orchestrator stopped", "error", err)
			os.Exit(1)
		}
	}()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           orc.HandleAPI(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		logger.Info("API server starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	ctxShut, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShut)
	_ = orc.Shutdown(ctxShut)
	logger.Info("shutdown complete")
}
