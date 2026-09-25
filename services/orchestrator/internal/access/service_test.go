package access

import (
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexastudio/nexacloud/pkg/model"
)

func TestClientIPUsesLastValidForwardedAddress(t *testing.T) {
	request := httptest.NewRequest("GET", "https://cloud.nexastudio.dev/", nil)
	request.Header.Set("X-Forwarded-For", "spoofed, 198.51.100.20")
	if actual := clientIP(request); actual != "198.51.100.20" {
		t.Fatalf("unexpected client IP %q", actual)
	}
}

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

func TestValidNodeAddress(t *testing.T) {
	for _, address := range []string{"141.11.165.20", "2001:db8::20", "10.0.0.12"} {
		if !validNodeAddress(address) {
			t.Fatalf("expected %s to be accepted", address)
		}
	}
	for _, address := range []string{"", "cloud.example.com", "127.0.0.1", "0.0.0.0", "224.0.0.1"} {
		if validNodeAddress(address) {
			t.Fatalf("expected %s to be rejected", address)
		}
	}
}

func TestValidSSHPublicKey(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if !validSSHPublicKey("ssh-ed25519 " + payload + " workstation") {
		t.Fatal("expected a valid Ed25519 public key")
	}
	for _, key := range []string{"", "ssh-dss " + payload, "ssh-ed25519 invalid", "ssh-ed25519 " + payload + "\ninjected"} {
		if validSSHPublicKey(key) {
			t.Fatalf("expected key %q to be rejected", key)
		}
	}
}
