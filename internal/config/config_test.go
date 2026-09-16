package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"APP_ADDR",
		"APP_LOG_LEVEL",
		"APP_ENVIRONMENT",
		"APP_VERSION",
		"APP_SHUTDOWN_TIMEOUT",
	} {
		if value, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, value) })
			os.Unsetenv(key)
		}
	}

	cfg := Load()

	if cfg.Environment != "development" {
		t.Errorf("environment = %q, want %q", cfg.Environment, "development")
	}
	if cfg.Version != "dev" {
		t.Errorf("version = %q, want %q", cfg.Version, "dev")
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("shutdown timeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
}

func TestLoadInvalidShutdownTimeoutUsesDefault(t *testing.T) {
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "invalid")

	cfg := Load()

	if cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Errorf("shutdown timeout = %v, want %v", cfg.ShutdownTimeout, defaultShutdownTimeout)
	}
}

func TestLoadNonPositiveShutdownTimeoutUsesDefault(t *testing.T) {
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "0s")

	cfg := Load()

	if cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Errorf("shutdown timeout = %v, want %v", cfg.ShutdownTimeout, defaultShutdownTimeout)
	}
}

func TestLoadPostgresSSLModeDefault(t *testing.T) {
	if value, ok := os.LookupEnv("POSTGRES_SSLMODE"); ok {
		t.Cleanup(func() { os.Setenv("POSTGRES_SSLMODE", value) })
		os.Unsetenv("POSTGRES_SSLMODE")
	}

	if got := Load().Postgres.SSLMode; got != "disable" {
		t.Errorf("sslmode = %q, want %q", got, "disable")
	}
}

func TestPostgresConfigDSNEscapesCredentials(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "p@ss:word",
		Database: "youthpreneur",
		SSLMode:  "disable",
	}

	want := "postgres://user:p%40ss%3Aword@localhost:5432/youthpreneur?sslmode=disable"
	if got := cfg.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("APP_ENVIRONMENT", "production")
	t.Setenv("APP_VERSION", "1.2.3")
	t.Setenv("APP_SHUTDOWN_TIMEOUT", "500ms")

	cfg := Load()

	if cfg.Environment != "production" {
		t.Errorf("environment = %q, want %q", cfg.Environment, "production")
	}
	if cfg.Version != "1.2.3" {
		t.Errorf("version = %q, want %q", cfg.Version, "1.2.3")
	}
	if cfg.ShutdownTimeout != 500*time.Millisecond {
		t.Errorf("shutdown timeout = %v, want %v", cfg.ShutdownTimeout, 500*time.Millisecond)
	}
}
