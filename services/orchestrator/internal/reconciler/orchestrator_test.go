package reconciler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLandingPage(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	orchestrator.HandleAPI().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected HTML content type, got %q", contentType)
	}
	if !strings.Contains(response.Body.String(), "NexaCloud") {
		t.Fatal("landing page does not contain the product name")
	}
}

func TestSecurityHeaders(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	orchestrator.HandleAPI().ServeHTTP(response, request)
	for _, header := range []string{"Content-Security-Policy", "Strict-Transport-Security", "X-Content-Type-Options", "X-Frame-Options", "Permissions-Policy"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("missing security header %s", header)
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	orchestrator.HandleAPI().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected health response: %s", response.Body.String())
	}
}

func TestStaticAsset(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()

	request := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	response := httptest.NewRecorder()
	orchestrator.HandleAPI().ServeHTTP(response, request)

	result := response.Result()
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusOK || !strings.Contains(string(body), "loadPlatformStatus") {
		t.Fatalf("static asset unavailable: status=%d", result.StatusCode)
	}
}

func TestProductPages(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()

	for _, path := range []string{"/", "/platform", "/infrastructure", "/pricing", "/security", "/download", "/roadmap", "/docs", "/login", "/register", "/dashboard", "/dashboard/servers", "/dashboard/infrastructure", "/dashboard/billing", "/dashboard/profile", "/staff"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			orchestrator.HandleAPI().ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("expected status 200 for %s, got %d", path, response.Code)
			}
			if !strings.Contains(response.Body.String(), "NexaCloud") {
				t.Fatalf("%s does not contain the product name", path)
			}
		})
	}
}

func TestSEOIndexationBoundaries(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()
	handler := orchestrator.HandleAPI()
	public := httptest.NewRecorder()
	handler.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/pricing", nil))
	for _, expected := range []string{`rel="canonical" href="https://cloud.nexastudio.dev/pricing"`, `name="robots" content="index,follow`, `property="og:title"`, `name="twitter:card"`} {
		if !strings.Contains(public.Body.String(), expected) {
			t.Fatalf("pricing metadata missing %q", expected)
		}
	}
	private := httptest.NewRecorder()
	handler.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if private.Header().Get("X-Robots-Tag") != "noindex, nofollow" || !strings.Contains(private.Body.String(), `content="noindex,nofollow"`) {
		t.Fatal("private dashboard must be explicitly excluded from indexing")
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound || missing.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal("unknown routes must return a noindex 404")
	}
}

func TestSEOInfrastructureEndpoints(t *testing.T) {
	orchestrator := &Orchestrator{mux: http.NewServeMux()}
	orchestrator.setupRoutes()
	handler := orchestrator.HandleAPI()
	for _, test := range []struct{ path, contentType, contains string }{{"/robots.txt", "text/plain", "/sitemap.xml"}, {"/sitemap.xml", "application/xml", "/security"}, {"/.well-known/security.txt", "text/plain", "nexastudio.dev/contact"}} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), test.contentType) || !strings.Contains(response.Body.String(), test.contains) {
			t.Fatalf("invalid SEO endpoint %s", test.path)
		}
	}
	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/features", nil))
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/platform" {
		t.Fatal("features alias must redirect directly to platform")
	}
}
