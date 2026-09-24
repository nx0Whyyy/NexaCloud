package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicRoutes(t *testing.T) {
	app := &Server{status: StatusResponse{Status: "operational", UpdatedAt: time.Now(), Components: []Component{}, Incidents: []Incident{}}}
	handler := app.Handler()
	for _, test := range []struct {
		path    string
		content string
	}{
		{path: "/healthz", content: `"status":"ok"`},
		{path: "/api/v1/status", content: `"status":"operational"`},
		{path: "/", content: "NexaCloud"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", test.path, response.Code)
		}
		if !strings.Contains(response.Body.String(), test.content) {
			t.Fatalf("%s response missing %q", test.path, test.content)
		}
		if response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("security headers missing on %s", test.path)
		}
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("1234567890"); got != "1234567" {
		t.Fatalf("unexpected short SHA: %s", got)
	}
}
