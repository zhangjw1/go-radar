package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSQLitePathFromURLResolvesRelativeToEnvDir(t *testing.T) {
	got, err := SQLitePathFromURL("sqlite:///./radar.db", filepath.Join("D:", "repo"))
	if err != nil {
		t.Fatalf("SQLitePathFromURL returned error: %v", err)
	}
	want := filepath.Clean(filepath.Join("D:", "repo", "radar.db"))
	if got != want {
		t.Fatalf("path mismatch: got %q want %q", got, want)
	}
}

func TestSQLitePathFromURLRejectsUnsupportedURL(t *testing.T) {
	if _, err := SQLitePathFromURL("postgres://example", "."); err == nil {
		t.Fatal("expected unsupported database URL to fail")
	}
}

func TestParseDatabaseURLSupportsPostgres(t *testing.T) {
	driver, value, err := ParseDatabaseURL("postgres://user:pass@127.0.0.1:5432/go_radar?sslmode=disable", ".")
	if err != nil {
		t.Fatalf("ParseDatabaseURL returned error: %v", err)
	}
	if driver != DBDriverPostgres {
		t.Fatalf("expected postgres driver, got %q", driver)
	}
	if value != "postgres://user:pass@127.0.0.1:5432/go_radar?sslmode=disable" {
		t.Fatalf("unexpected postgres value: %q", value)
	}
}

func TestLoadWorksWithoutEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("GO_RADAR_ENV_FILE", "none")
	t.Setenv("GO_RADAR_PORT", "")

	settings, err := Load()
	if err != nil {
		t.Fatalf("Load returned error without .env: %v", err)
	}
	if settings.EnvPath != "" {
		t.Fatalf("expected empty EnvPath without .env, got %q", settings.EnvPath)
	}
	if settings.DatabasePath != filepath.Join(tempDir, "radar.db") {
		t.Fatalf("unexpected database path: %q", settings.DatabasePath)
	}
	if settings.DatabaseDriver != DBDriverSQLite {
		t.Fatalf("expected sqlite driver, got %q", settings.DatabaseDriver)
	}
	if !settings.AutoMigrate {
		t.Fatal("expected auto migration to be enabled by default")
	}
}

func TestLoadSupportsExplicitEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("GO_RADAR_PORT=19090\nDATABASE_URL=sqlite:///./local.db\n"), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	t.Setenv("GO_RADAR_ENV_FILE", envPath)
	restorePort := clearEnvForTest(t, "GO_RADAR_PORT")
	defer restorePort()
	restoreDatabaseURL := clearEnvForTest(t, "DATABASE_URL")
	defer restoreDatabaseURL()

	settings, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if settings.Port != "19090" {
		t.Fatalf("expected port from explicit env file, got %q", settings.Port)
	}
	if settings.DatabasePath != filepath.Join(tempDir, "local.db") {
		t.Fatalf("unexpected database path: %q", settings.DatabasePath)
	}
	if settings.DatabaseDriver != DBDriverSQLite {
		t.Fatalf("expected sqlite driver, got %q", settings.DatabaseDriver)
	}
}

func clearEnvForTest(t *testing.T, key string) func() {
	t.Helper()
	value, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	return func() {
		if existed {
			_ = os.Setenv(key, value)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}
