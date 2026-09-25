package auth

import (
	"net/http/httptest"
	"testing"
	"time"
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

func TestPasswordPolicy(t *testing.T) {
	if message := passwordPolicy("short", "user", "user@example.com"); message == "" {
		t.Fatal("short password should be rejected")
	}
	if message := passwordPolicy("NexaCloud!Secure2026", "nexa", "person@example.com"); message == "" {
		t.Fatal("password containing username should be rejected")
	}
	if message := passwordPolicy("Cloud!River92Stone", "nexa", "person@example.com"); message != "" {
		t.Fatalf("strong password should be accepted: %s", message)
	}
}

func TestLimiter(t *testing.T) {
	limit := newLimiter(2, time.Minute)
	now := time.Now()
	if !limit.allow("key", now) || !limit.allow("key", now) || limit.allow("key", now) {
		t.Fatal("limiter should reject requests above its threshold")
	}
	if !limit.allow("key", now.Add(2*time.Minute)) {
		t.Fatal("limiter should reset after its window")
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
	request.Header.Del("Origin")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	if validOrigin(request) {
		t.Fatal("cross-site browser request without origin should be rejected")
	}
}

func TestRoleHierarchy(t *testing.T) {
	if canManageRole("admin", "user", "owner") || canManageRole("admin", "owner", "user") {
		t.Fatal("admins must not create or modify owners")
	}
	if !canManageRole("admin", "user", "moderator") {
		t.Fatal("admins should manage non-owner roles")
	}
	if !canManageRole("owner", "owner", "admin") {
		t.Fatal("owners should manage every role")
	}
}
