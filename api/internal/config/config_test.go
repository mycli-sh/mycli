package config

import (
	"os"
	"testing"
)

func TestBuildDatabaseURL(t *testing.T) {
	// Helper to clear all DB-related env vars before each subtest.
	clearEnv := func(t *testing.T) {
		t.Helper()
		for _, key := range []string{"DATABASE_URL", "DB_HOST", "DB_USER", "DB_PASSWORD", "DB_PORT", "DB_NAME", "DB_SSLMODE"} {
			t.Setenv(key, "")
			_ = os.Unsetenv(key)
		}
	}

	t.Run("default when nothing set", func(t *testing.T) {
		clearEnv(t)
		got := buildDatabaseURL()
		want := "postgres://mycli:mycli@localhost:5432/mycli_dev?sslmode=disable"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("DATABASE_URL takes priority", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DATABASE_URL", "postgres://custom:pass@dbhost:9999/mydb")
		t.Setenv("DB_HOST", "ignored")
		got := buildDatabaseURL()
		if got != "postgres://custom:pass@dbhost:9999/mydb" {
			t.Errorf("DATABASE_URL should take priority, got %q", got)
		}
	})

	t.Run("individual vars construct DSN", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_HOST", "prod-db")
		t.Setenv("DB_USER", "admin")
		t.Setenv("DB_PASSWORD", "s3cret")
		t.Setenv("DB_PORT", "5433")
		t.Setenv("DB_NAME", "production")
		t.Setenv("DB_SSLMODE", "require")
		got := buildDatabaseURL()
		want := "postgres://admin:s3cret@prod-db:5433/production?sslmode=require"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("partial individual vars use defaults for the rest", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DB_HOST", "remotehost")
		got := buildDatabaseURL()
		want := "postgres://mycli:mycli@remotehost:5432/mycli_dev?sslmode=disable"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
