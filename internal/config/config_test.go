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
