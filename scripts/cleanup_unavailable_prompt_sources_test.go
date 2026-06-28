package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupOptionsLoadDotEnvDatabaseConfig(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	oldDatabaseDriver, hadDatabaseDriver := os.LookupEnv("DATABASE_DRIVER")
	oldStorageDriver, hadStorageDriver := os.LookupEnv("STORAGE_DRIVER")
	oldDatabaseDSN, hadDatabaseDSN := os.LookupEnv("DATABASE_DSN")
	_ = os.Unsetenv("DATABASE_DRIVER")
	_ = os.Unsetenv("STORAGE_DRIVER")
	_ = os.Unsetenv("DATABASE_DSN")
	t.Cleanup(func() {
		restoreEnv("DATABASE_DRIVER", oldDatabaseDriver, hadDatabaseDriver)
		restoreEnv("STORAGE_DRIVER", oldStorageDriver, hadStorageDriver)
		restoreEnv("DATABASE_DSN", oldDatabaseDSN, hadDatabaseDSN)
	})
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DATABASE_DRIVER=mysql\nDATABASE_DSN=user:pass@tcp(127.0.0.1:3306)/infinite_canvas\n"), 0644); err != nil {
		t.Fatal(err)
	}

	options := defaultCleanupOptions()

	if options.driver != "mysql" {
		t.Fatalf("driver = %q, want mysql", options.driver)
	}
	if options.dsn != "user:pass@tcp(127.0.0.1:3306)/infinite_canvas" {
		t.Fatalf("dsn = %q, want .env DSN", options.dsn)
	}
}

func restoreEnv(key string, value string, ok bool) {
	if !ok {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, value)
}
