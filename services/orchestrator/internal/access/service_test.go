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
