package access

import (
	"strings"
	"testing"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
)

func TestNewLicenseKeyIsDisplayableAndHashed(t *testing.T) {
	key, hash, err := NewLicenseKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "NX-") || len(key) != 22 {
		t.Fatalf("unexpected license format: %q", key)
	}
	if len(hash) != 64 || strings.Contains(hash, key) {
		t.Fatal("license must be represented by an opaque SHA-256 hash")
	}
}

func TestLicenseUsable(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	if !LicenseUsable(model.License{Status: "ACTIVE", ExpiresAt: &future}, now) {
		t.Fatal("active unexpired license should be usable")
	}
	if LicenseUsable(model.License{Status: "ACTIVE", ExpiresAt: &past}, now) {
		t.Fatal("expired license should not be usable")
	}
	if LicenseUsable(model.License{Status: "REVOKED"}, now) {
		t.Fatal("revoked license should not be usable")
	}
}

func TestDeviceCodesAreDisplayableAndOpaque(t *testing.T) {
	code, err := randomDisplayCode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "NXA-") || len(code) != 11 {
		t.Fatalf("unexpected device code format: %q", code)
	}
	token, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 40 || hashSecret(token) == token {
		t.Fatal("device polling token must be high entropy and stored hashed")
	}
}
