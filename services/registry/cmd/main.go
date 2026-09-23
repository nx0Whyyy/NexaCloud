package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
	"github.com/nexastudio/nexacloud/services/registry/internal/config"
	"github.com/nexastudio/nexacloud/services/registry/internal/discovery"
	"github.com/nexastudio/nexacloud/services/registry/internal/routing"
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

	reg := discovery.New(db, nc, logger)
	rt := routing.New(db, logger)

	sub, err := nc.QueueSubscribe("registry.+", "registry", reg.Handle)
	if err != nil {
		logger.Error("subscribe error", "error", err)
		os.Exit(1)
	}
	defer sub.Drain()

	sub2, err := nc.QueueSubscribe("routing.request", "registry", rt.Handle)
	if err != nil {
		logger.Error("subscribe error", "error", err)
		os.Exit(1)
	}
	defer sub2.Drain()

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.APIPort),
		Handler: newHandler(db, reg, rt),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("registry API starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	logger.Info("registry started", "version", version)
	<-ctx.Done()
	logger.Info("registry shutting down")
	_ = srv.Shutdown(context.Background())
}

func newHandler(db *gorm.DB, reg *discovery.Registry, rt *routing.Router) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/routing", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		service := r.URL.Query().Get("service")
		if service == "" {
			http.Error(w, "service query param required", http.StatusBadRequest)
			return
		}

		instance := rt.SelectInstance(service)
		w.Header().Set("Content-Type", "application/json")
		if instance != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"service": instance.ServiceName,
				"address": instance.Address,
				"port":    instance.Port,
				"ready":   instance.Ready,
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{"error": "no instance available"})
		}
	})

	mux.HandleFunc("/api/v1/services", func(w http.ResponseWriter, r *http.Request) {
		var services []model.RegisteredService
		db.Find(&services)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
	})

	return mux
}
