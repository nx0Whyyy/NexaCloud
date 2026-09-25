package docker

import (
	"strings"
	"testing"
)

func TestCleanDataTarget(t *testing.T) {
	tests := map[string]string{".": "/data", "server.properties": "/data/server.properties", "plugins/NexaLink/config.yml": "/data/plugins/NexaLink/config.yml"}
	for input, expected := range tests {
		actual, err := cleanDataTarget(input)
		if err != nil || actual != expected {
			t.Fatalf("cleanDataTarget(%q) = %q, %v", input, actual, err)
		}
	}
	for _, input := range []string{"../etc/passwd", "plugins/../../etc", "/etc/passwd", `plugins\secret`, "bad\x00path"} {
		if _, err := cleanDataTarget(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}

func TestBoundedOutput(t *testing.T) {
	if _, err := boundedOutput(strings.Repeat("x", maxFileResult+1), nil); err == nil {
		t.Fatal("oversized output must be rejected")
	}
}
