package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestArgon2PasswordRoundTrip(t *testing.T) {
	hash, err := hashPassword("a sufficiently long password")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected Argon2id hash, got %q", hash)
	}
	valid, legacy := verifyPassword(hash, "a sufficiently long password")
	if !valid || legacy {
		t.Fatal("Argon2id password should verify without legacy migration")
	}
	if valid, _ := verifyPassword(hash, "wrong password"); valid {
		t.Fatal("wrong password must not verify")
	}
}

func TestLegacyBcryptPasswordIsAcceptedForMigration(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("legacy password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	valid, legacy := verifyPassword(string(hash), "legacy password")
	if !valid || !legacy {
		t.Fatal("valid bcrypt hash should request migration")
	}
}
