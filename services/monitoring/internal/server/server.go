package server

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nexastudio/nexacloud/services/monitoring/internal/config"
	"gorm.io/gorm"
)

//go:embed web/dist
var frontend embed.FS

type Server struct {
	cfg    *config.Config
	db     *gorm.DB
	logger *slog.Logger
	github *GitHubClient
	client *http.Client
	mu     sync.RWMutex
	status StatusResponse
}

type probe struct {
	id          string
	name        string
	description string
	check       func(context.Context) error
}

func New(cfg *config.Config, db *gorm.DB, logger *slog.Logger) (*Server, error) {
	if err := db.AutoMigrate(&Snapshot{}, &Incident{}); err != nil {
		return nil, err
	}
	return &Server{
		cfg: cfg, db: db, logger: logger,
		github: NewGitHubClient(cfg.GitHubRepo, cfg.GitHubToken),
		client: &http.Client{Timeout: 4 * time.Second},
		status: StatusResponse{Status: "initializing", Message: "Vérification des systèmes en cours", Components: []Component{}, Incidents: []Incident{}},
	}, nil
}

func (s *Server) Run(ctx context.Context) {
	s.checkAll(ctx)
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAll(ctx)
		}
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/status/history", s.handleHistory)
	mux.HandleFunc("GET /api/v1/incidents", s.handleIncidents)
	mux.HandleFunc("GET /api/v1/changelog", s.handleChangelog)
	mux.HandleFunc("GET /api/v1/changelog/feed", s.handleChangelog)
	mux.HandleFunc("/", s.handleFrontend)
	return securityHeaders(mux)
}

func (s *Server) probes() []probe {
	return []probe{
		{id: "control-plane", name: "Control plane", description: "Orchestration et API NexaCloud", check: s.httpProbe(s.cfg.Orchestrator)},
		{id: "registry", name: "Registry", description: "Découverte et routage des services", check: s.httpProbe(s.cfg.Registry)},
		{id: "postgresql", name: "PostgreSQL", description: "Persistance de la plateforme", check: func(ctx context.Context) error {
			sqlDB, err := s.db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		}},
		{id: "redis", name: "Redis", description: "Cache et état temps réel", check: redisProbe(s.cfg.RedisAddress)},
		{id: "nats", name: "NATS JetStream", description: "Bus d’événements du réseau", check: tcpProbe(s.cfg.NATSAddress)},
	}
}

func (s *Server) checkAll(ctx context.Context) {
	checks := s.probes()
	components := make([]Component, len(checks))
	var wait sync.WaitGroup
	for i, item := range checks {
		wait.Add(1)
		go func(index int, candidate probe) {
			defer wait.Done()
			checkCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			started := time.Now()
			err := candidate.check(checkCtx)
			component := Component{ID: candidate.id, Name: candidate.name, Description: candidate.description, Status: "operational", LatencyMS: time.Since(started).Milliseconds(), CheckedAt: time.Now().UTC()}
			if err != nil {
				component.Status = "outage"
				component.Error = publicError(err)
			}
			component.Uptime24H = s.uptime(candidate.id, component.Status)
			components[index] = component
		}(i, item)
	}
	wait.Wait()

	overall := "operational"
	message := "Tous les systèmes sont opérationnels"
	for _, component := range components {
		if component.Status != "operational" {
			overall = "major_outage"
			message = "Une interruption affecte certains systèmes"
		}
		s.record(component)
	}
	var incidents []Incident
	s.db.Order("started_at DESC").Limit(20).Find(&incidents)
	s.mu.Lock()
	s.status = StatusResponse{Status: overall, Message: message, UpdatedAt: time.Now().UTC(), Components: components, Incidents: incidents}
	s.mu.Unlock()
	if err := s.db.Where("checked_at < ?", time.Now().Add(-90*24*time.Hour)).Delete(&Snapshot{}).Error; err != nil {
		s.logger.Warn("snapshot cleanup failed", "error", err)
	}
}

func (s *Server) record(component Component) {
	now := time.Now().UTC()
	if err := s.db.Create(&Snapshot{Component: component.ID, Status: component.Status, LatencyMS: component.LatencyMS, CheckedAt: now}).Error; err != nil {
		s.logger.Warn("snapshot insert failed", "component", component.ID, "error", err)
	}
	var open Incident
	err := s.db.Where("component = ? AND status = ?", component.ID, "investigating").First(&open).Error
	if component.Status == "outage" && errors.Is(err, gorm.ErrRecordNotFound) {
		message := component.Error
		_ = s.db.Create(&Incident{Component: component.ID, Title: component.Name + " indisponible", Status: "investigating", StartedAt: now, LastMessage: message}).Error
	}
	if component.Status == "operational" && err == nil {
		open.Status = "resolved"
		open.ResolvedAt = &now
		open.LastMessage = "Le service est de nouveau opérationnel."
		_ = s.db.Save(&open).Error
	}
}

func (s *Server) uptime(component, current string) float64 {
	var total, healthy int64
	since := time.Now().Add(-24 * time.Hour)
	s.db.Model(&Snapshot{}).Where("component = ? AND checked_at >= ?", component, since).Count(&total)
	s.db.Model(&Snapshot{}).Where("component = ? AND checked_at >= ? AND status = ?", component, since, "operational").Count(&healthy)
	if total == 0 {
		if current == "operational" {
			return 100
		}
		return 0
	}
	return float64(healthy) / float64(total) * 100
}

func (s *Server) httpProbe(target string) func(context.Context) error {
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		response, err := s.client.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("HTTP %d", response.StatusCode)
		}
		return nil
	}
}

func tcpProbe(address string) func(context.Context) error {
	return func(ctx context.Context) error {
		connection, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", address)
		if err == nil {
			_ = connection.Close()
		}
		return err
	}
}

func redisProbe(address string) func(context.Context) error {
	return func(ctx context.Context) error {
		connection, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", address)
		if err != nil {
			return err
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := connection.Write([]byte("PING\r\n")); err != nil {
			return err
		}
		line, err := bufio.NewReader(connection).ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(line) != "+PONG" {
			return errors.New("unexpected response")
		}
		return nil
	}
}

func publicError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "Délai de réponse dépassé"
	}
	return "Sonde de disponibilité en échec"
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	var snapshots []Snapshot
	s.db.Where("checked_at >= ?", time.Now().Add(-24*time.Hour)).Order("checked_at ASC").Limit(5000).Find(&snapshots)
	writeJSON(w, http.StatusOK, map[string]any{"period": "24h", "snapshots": snapshots})
}

func (s *Server) handleIncidents(w http.ResponseWriter, _ *http.Request) {
	var incidents []Incident
	s.db.Order("started_at DESC").Limit(100).Find(&incidents)
	writeJSON(w, http.StatusOK, map[string]any{"incidents": incidents})
}

func (s *Server) handleChangelog(w http.ResponseWriter, r *http.Request) {
	result, err := s.github.Changelog(r.Context())
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": "GitHub temporairement indisponible"})
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSONStatus(w, http.StatusOK, result)
}

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	files, _ := fs.Sub(frontend, "web/dist")
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" {
		if _, err := fs.Stat(files, path); err == nil {
			http.FileServer(http.FS(files)).ServeHTTP(w, r)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	content, _ := fs.ReadFile(files, "index.html")
	_, _ = w.Write(content)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
