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

	for _, path := range []string{"/platform", "/login", "/register", "/dashboard", "/staff"} {
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
