package reconciler

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nexastudio/nexacloud/pkg/model"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/access"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/auth"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/config"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/crashloop"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/lifecycle"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/mailer"
	"github.com/nexastudio/nexacloud/services/orchestrator/internal/shift"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

//go:embed web
var webFiles embed.FS

type Orchestrator struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *gorm.DB
	nc     *nats.Conn
	js     nats.JetStreamContext

	lifecycle *lifecycle.Manager
	shift     *shift.Shifter
	crashloop *crashloop.Detector
	auth      *auth.Service
	access    *access.Service

	wg     sync.WaitGroup
	mux    *http.ServeMux
	stopCh chan struct{}
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
		cfg:    cfg,
		logger: logger,
		db:     db,
		nc:     nc,
		js:     js,
		stopCh: make(chan struct{}),
		mux:    http.NewServeMux(),
	}

	o.lifecycle = lifecycle.New(db, nc, logger)
	o.shift = shift.New(db, nc, logger)
	o.crashloop = crashloop.New(db, logger)
	o.access = access.New(db)
	mailService := mailer.New(mailer.Config{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.SMTPFrom, FromName: cfg.SMTPFromName})
	o.auth = auth.New(db, o.access.ProvisionUser, mailService, cfg.PublicURL)

	if err := o.migrate(); err != nil {
		return nil, err
	}
	if err := o.auth.EnsureAdmin(cfg.AdminUsername, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		return nil, err
	}
	o.setupRoutes()

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
	if err := o.access.MarkStaleNodes(model.Now()); err != nil {
		o.logger.Error("mark stale nodes failed", "error", err)
	}
	o.lifecycle.Reconcile(ctx)
	o.shift.Reconcile(ctx)
	o.crashloop.Check(ctx)
}

func (o *Orchestrator) setupRoutes() {
	staticFiles, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}

	o.mux.Handle("/assets/", http.FileServer(http.FS(staticFiles)))
	o.mux.Handle("/downloads/", http.FileServer(http.FS(staticFiles)))
	o.mux.Handle("/site.webmanifest", http.FileServer(http.FS(staticFiles)))
	o.mux.HandleFunc("GET /robots.txt", serveText("text/plain; charset=utf-8", robotsText()))
	o.mux.HandleFunc("GET /sitemap.xml", serveText("application/xml; charset=utf-8", sitemapXML()))
	o.mux.HandleFunc("GET /.well-known/security.txt", serveText("text/plain; charset=utf-8", "Contact: https://nexastudio.dev/contact\nExpires: 2027-09-25T00:00:00Z\nPreferred-Languages: fr, en\nCanonical: https://cloud.nexastudio.dev/.well-known/security.txt\n"))
	o.mux.HandleFunc("GET /install/nexa-agent.sh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline; filename=nexa-agent.sh")
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, staticFiles, "install-nexa-agent.sh")
	})
	o.mux.HandleFunc("/healthz", o.handleHealth)
	o.mux.HandleFunc("/api/v1/platform", o.handlePlatform)
	if o.auth != nil {
		o.auth.RegisterRoutes(o.mux)
		o.access.RegisterRoutes(o.mux, o.auth.CurrentUser)
	}
	o.mux.HandleFunc("GET /platform", o.servePage("platform.html"))
	o.mux.HandleFunc("GET /features", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/platform", http.StatusMovedPermanently)
	})
	o.mux.HandleFunc("GET /infrastructure", o.servePage("infrastructure.html"))
	o.mux.HandleFunc("GET /pricing", o.servePage("pricing.html"))
	o.mux.HandleFunc("GET /security", o.servePage("security.html"))
	o.mux.HandleFunc("GET /download", o.servePage("download.html"))
	o.mux.HandleFunc("GET /roadmap", o.servePage("roadmap.html"))
	o.mux.HandleFunc("GET /docs", o.servePage("docs.html"))
	o.mux.HandleFunc("GET /login", o.servePage("login.html"))
	o.mux.HandleFunc("GET /register", o.servePage("register.html"))
	o.mux.HandleFunc("GET /verify", o.servePage("verify.html"))
	o.mux.HandleFunc("GET /dashboard", o.servePage("dashboard.html"))
	o.mux.HandleFunc("GET /dashboard/infrastructure", o.servePage("dashboard-infrastructure.html"))
	o.mux.HandleFunc("GET /dashboard/servers", o.servePage("dashboard-servers.html"))
	o.mux.HandleFunc("GET /dashboard/billing", o.servePage("dashboard-billing.html"))
	o.mux.HandleFunc("GET /dashboard/profile", o.servePage("dashboard-profile.html"))
	o.mux.HandleFunc("GET /staff", o.servePage("staff.html"))
	o.mux.HandleFunc("/", o.handleRoot)
}

func (o *Orchestrator) migrate() error {
	if err := o.auth.Migrate(); err != nil {
		return err
	}
	return o.access.Migrate()
}

func (o *Orchestrator) servePage(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		page, exists := seoPages[path]
		if !exists && (strings.HasPrefix(path, "/dashboard") || path == "/staff") {
			page = seoPage{Title: "NexaCloud Control Plane", Description: "Private NexaCloud operator workspace.", Index: false}
			exists = true
		}
		data, err := fs.ReadFile(webFiles, "web/"+name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if exists && !page.Index {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		if exists {
			data = []byte(renderSEOPage(string(data), path, page))
		}
		_, _ = w.Write(data)
	}
}

func (o *Orchestrator) Shutdown(ctx context.Context) error {
	close(o.stopCh)
	o.nc.Drain()
	return nil
}

func (o *Orchestrator) HandleAPI() http.Handler {
	return securityHeaders(o.mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/dashboard") || r.URL.Path == "/staff" {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'; object-src 'none'; img-src 'self' data:; connect-src 'self' https://status.nexastudio.dev; font-src 'self'; style-src 'self'; script-src 'self'")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (o *Orchestrator) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!doctype html><html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow"><title>Page introuvable | NexaCloud</title><link rel="stylesheet" href="/assets/styles.css"></head><body class="inner-page"><main><section class="page-hero"><p class="eyebrow"><span>404</span> Page introuvable</p><h1>Cette route n'existe pas.</h1><p>Retournez au produit, à la documentation ou à votre espace NexaCloud.</p><p><a class="button button-primary" href="/">Accueil</a> <a class="button button-ghost" href="/docs">Documentation</a></p></section></main></body></html>`))
		return
	}
	o.servePage("index.html")(w, r)
}

func (o *Orchestrator) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "orchestrator",
		"status":  "ok",
	})
}

func (o *Orchestrator) handlePlatform(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	postgresStatus := "unavailable"
	if sqlDB, err := o.db.DB(); err == nil && sqlDB.PingContext(ctx) == nil {
		postgresStatus = "operational"
	}

	natsStatus := "unavailable"
	if o.nc.IsConnected() {
		natsStatus = "operational"
	}

	redisStatus := "unavailable"
	if redisHealthy(o.cfg.RedisHost, o.cfg.RedisPort) {
		redisStatus = "operational"
	}

	status := "operational"
	if postgresStatus != status || redisStatus != status || natsStatus != status {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": status,
		"components": []map[string]string{
			{"name": "Control plane", "status": "operational"},
			{"name": "PostgreSQL", "status": postgresStatus},
			{"name": "Redis", "status": redisStatus},
			{"name": "NATS JetStream", "status": natsStatus},
		},
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func redisHealthy(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return false
	}

	response, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(response) == "+PONG"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func staticFiles() fs.FS {
	files, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	return files
}
