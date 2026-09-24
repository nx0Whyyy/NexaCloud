package auth

import (
	"net/http/httptest"
	"testing"
)

func TestTokenHashIsStableAndOpaque(t *testing.T) {
	first := tokenHash("session-token")
	second := tokenHash("session-token")
	if first != second {
		t.Fatal("token hash must be stable")
	}
	if first == "session-token" || len(first) != 64 {
		t.Fatal("session token must be stored as a SHA-256 hash")
	}
}

func TestValidOrigin(t *testing.T) {
	request := httptest.NewRequest("POST", "https://cloud.nexastudio.dev/api/v1/auth/login", nil)
	request.Host = "cloud.nexastudio.dev"
	request.Header.Set("Origin", "https://cloud.nexastudio.dev")
	if !validOrigin(request) {
		t.Fatal("same-origin request should be accepted")
	}
	request.Header.Set("Origin", "https://example.com")
	if validOrigin(request) {
		t.Fatal("cross-origin request should be rejected")
	}
}
