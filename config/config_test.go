package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeDockerSQLiteDSNUsesMountedDataDir(t *testing.T) {
	root := t.TempDir()
	appDataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		t.Fatal(err)
	}
	Cfg = Config{StorageDriver: "sqlite", DatabaseDSN: "data/infinite-canvas.db?_pragma=busy_timeout(5000)"}

	normalizeDockerSQLiteDSN(appDataDir)

	want := filepath.Join(root, "data", "infinite-canvas.db") + "?_pragma=busy_timeout(5000)"
	if Cfg.DatabaseDSN != want {
		t.Fatalf("DatabaseDSN = %q, want %q", Cfg.DatabaseDSN, want)
	}
}

func TestNormalizeDockerSQLiteDSNLeavesLocalPathWithoutMountedDataDir(t *testing.T) {
	Cfg = Config{StorageDriver: "sqlite", DatabaseDSN: "data/infinite-canvas.db"}

	normalizeDockerSQLiteDSN(filepath.Join(t.TempDir(), "missing-data"))

	if Cfg.DatabaseDSN != "data/infinite-canvas.db" {
		t.Fatalf("DatabaseDSN = %q, want relative local path", Cfg.DatabaseDSN)
	}
}

func TestNormalizeDatabaseDriverPrefersDatabaseDriver(t *testing.T) {
	Cfg = Config{DatabaseDriver: "mysql", StorageDriver: "sqlite"}

	normalizeDatabaseDriver()

	if Cfg.StorageDriver != "mysql" {
		t.Fatalf("StorageDriver = %q, want mysql", Cfg.StorageDriver)
	}
}

func TestNormalizeDockerSQLiteDSNLeavesMySQLDSN(t *testing.T) {
	root := t.TempDir()
	appDataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		t.Fatal(err)
	}
	Cfg = Config{DatabaseDriver: "mysql", StorageDriver: "sqlite", DatabaseDSN: "user:pass@tcp(127.0.0.1:3306)/infinite_canvas?parseTime=true"}
	normalizeDatabaseDriver()

	normalizeDockerSQLiteDSN(appDataDir)

	want := "user:pass@tcp(127.0.0.1:3306)/infinite_canvas?parseTime=true"
	if Cfg.DatabaseDSN != want {
		t.Fatalf("DatabaseDSN = %q, want mysql dsn unchanged", Cfg.DatabaseDSN)
	}
}
