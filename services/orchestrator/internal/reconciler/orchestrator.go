package reconciler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/config"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/lifecycle"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/shift"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/crashloop"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Orchestrator struct {
	cfg     *config.Config
	logger  *slog.Logger
	db      *gorm.DB
	nc      *nats.Conn
	js      nats.JetStreamContext

	lifecycle    *lifecycle.Manager
	shift        *shift.Shifter
	crashloop    *crashloop.Detector

	wg      sync.WaitGroup
	mux     *http.ServeMux
	stopCh  chan struct{}
}

func New(ctx context.Context, logger *slog.Logger, cfg *config.Config) (*Orchestrator, error) {
	dsn := "host=" + cfg.DBHost +
		" port=" + strconv.Itoa(cfg.DBPort) +
		" user=" + cfg.DBUser +
		" password=" + cfg.DBPassword +
		" dbname=" + cfg.DBName +
		" sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	_ = db

	nc, err := nats.Connect(cfg.NATSUrl)
	if err != nil {
		return nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	_ = js

	o := &Orchestrator{
		cfg:      cfg,
		logger:   logger,
		db:       db,
		nc:       nc,
		js:       js,
		stopCh:   make(chan struct{}),
		mux:      http.NewServeMux(),
	}

	o.lifecycle = lifecycle.New(db, nc, logger)
	o.shift = shift.New(db, nc, logger)
	o.crashloop = crashloop.New(db, logger)

	o.setupRoutes()
	o.migrate()

	return o, nil
}

func (o *Orchestrator) Run(ctx context.Context) error {
	ticker := time.NewTicker(o.cfg.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.stopCh:
			return nil
		case <-ticker.C:
			o.reconcile(ctx)
		}
	}
}

func (o *Orchestrator) reconcile(ctx context.Context) {
	o.lifecycle.Reconcile(ctx)
	o.shift.Reconcile(ctx)
	o.crashloop.Check(ctx)
}

func (o *Orchestrator) setupRoutes() {
	o.mux.HandleFunc("/", o.handleRoot)
}

func (o *Orchestrator) migrate() {
	_ = o.db
}

func (o *Orchestrator) Shutdown(ctx context.Context) error {
	close(o.stopCh)
	o.nc.Drain()
	return nil
}

func (o *Orchestrator) HandleAPI() http.Handler {
	return o.mux
}

func (o *Orchestrator) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"service":"orchestrator","status":"ok"}`))
}
