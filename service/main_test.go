package service

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/repository"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "infinite-canvas-service-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	config.Cfg = config.Config{StorageDriver: "sqlite", DatabaseDSN: filepath.Join(dir, "test.db"), JWTSecret: "test-secret", JWTExpireHours: 168, VerificationProvider: "noop"}
	if _, err := repository.DB(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
