package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
)

func TestRegisterRequiresEmailVerificationCode(t *testing.T) {
	previousConfig := config.Cfg
	config.Cfg = config.Config{
		StorageDriver:        "sqlite",
		DatabaseDSN:          filepath.Join(t.TempDir(), "test.db"),
		JWTSecret:            "test-secret",
		JWTExpireHours:       168,
		VerificationProvider: "noop",
	}
	t.Cleanup(func() {
		config.Cfg = previousConfig
	})

	email := "Designer.Verify@example.com"
	if _, err := Register("verify-user-missing", "password123", email, ""); err == nil || !strings.Contains(err.Error(), "验证码") {
		t.Fatalf("Register without code error = %v, want verification code error", err)
	}

	issued, err := RequestVerificationCode(email, "register")
	if err != nil {
		t.Fatalf("RequestVerificationCode: %v", err)
	}
	if issued.ExpiresInSeconds <= 0 || issued.DebugCode == "" {
		t.Fatalf("issued = %#v, want ttl and debug code for noop provider", issued)
	}

	session, err := Register("verify-user-ok", "password123", email, issued.DebugCode)
	if err != nil {
		t.Fatalf("Register with code: %v", err)
	}
	if session.User.Email != strings.ToLower(email) {
		t.Fatalf("session email = %q, want normalized email", session.User.Email)
	}

	if _, err := Register("verify-user-reuse", "password123", email, issued.DebugCode); err == nil {
		t.Fatal("Register reused code error = nil, want error")
	}
}
