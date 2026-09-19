package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolateConfig runs the test from an empty temporary directory so a developer
// `.env` in the repository root is never read, and clears known keys so the
// process environment cannot leak between tests.
func isolateConfig(t *testing.T) {
	t.Helper()

	t.Chdir(t.TempDir())
	if err := os.WriteFile(".env", []byte("APP_ENV=development\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	for _, key := range []string{
		"APP_ADDR",
		"APP_LOG_LEVEL",
		"APP_ENV",
		"APP_VERSION",
		"APP_SHUTDOWN_TIMEOUT",
		"APP_AUTH_SECRET",
		"APP_AUTH_ACCESS_TOKEN_TTL",
		"APP_AUTH_REFRESH_TOKEN_TTL",
		"APP_CORS_ALLOWED_ORIGINS",
		"APP_RATE_LIMIT_LOGIN_PER_MINUTE",
		"APP_RATE_LIMIT_REFRESH_PER_MINUTE",
		"APP_RATE_LIMIT_GENERAL_PER_MINUTE",
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DB",
		"POSTGRES_SSLMODE",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	isolateConfig(t)
	t.Setenv("APP_AUTH_SECRET", "a-strong-secret-value-that-is-long-enough")
	t.Setenv("APP_ENV", "production")
	t.Setenv("POSTGRES_HOST", "db.internal")

	cfg := Load()

	if cfg.Auth.Secret != "a-strong-secret-value-that-is-long-enough" {
		t.Errorf("secret = %q, want environment value", cfg.Auth.Secret)
	}
	if cfg.Environment != "production" {
		t.Errorf("environment = %q, want %q", cfg.Environment, "production")
	}
	if cfg.Postgres.Host != "db.internal" {
		t.Errorf("postgres host = %q, want %q", cfg.Postgres.Host, "db.internal")
	}
	if cfg.Addr != ":8080" {
		t.Errorf("addr = %q, want default %q", cfg.Addr, ":8080")
	}
}

func TestLoadRejectsMalformedDotEnv(t *testing.T) {
	isolateConfig(t)
	if err := os.WriteFile(filepath.Join(".", ".env"), []byte("APP_ADDR\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Error("Load() did not panic on malformed .env")
		}
	}()

	Load()
}

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"APP_ADDR",
		"APP_LOG_LEVEL",
		"APP_ENV",
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

func TestLoadSecureCookiesFollowsEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	if !Load().SecureCookies {
		t.Error("secure cookies = false, want true in production")
	}

	t.Setenv("APP_ENV", "development")
	if Load().SecureCookies {
		t.Error("secure cookies = true, want false outside production")
	}
}

func TestLoadAuthDefaults(t *testing.T) {
	for _, key := range []string{
		"APP_AUTH_SECRET",
		"APP_AUTH_ACCESS_TOKEN_TTL",
		"APP_AUTH_REFRESH_TOKEN_TTL",
	} {
		if value, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, value) })
			os.Unsetenv(key)
		}
	}

	cfg := Load()

	if cfg.Auth.Secret != "" {
		t.Errorf("secret = %q, want empty when unset", cfg.Auth.Secret)
	}
	if cfg.Auth.AccessTokenTTL != defaultAccessTTL {
		t.Errorf("access ttl = %v, want %v", cfg.Auth.AccessTokenTTL, defaultAccessTTL)
	}
	if cfg.Auth.RefreshTokenTTL != defaultRefreshTTL {
		t.Errorf("refresh ttl = %v, want %v", cfg.Auth.RefreshTokenTTL, defaultRefreshTTL)
	}
}

func TestLoadAuthOverrides(t *testing.T) {
	t.Setenv("APP_AUTH_SECRET", "a-strong-secret-value-that-is-long-enough")
	t.Setenv("APP_AUTH_ACCESS_TOKEN_TTL", "1m")
	t.Setenv("APP_AUTH_REFRESH_TOKEN_TTL", "24h")

	cfg := Load()

	if cfg.Auth.Secret != "a-strong-secret-value-that-is-long-enough" {
		t.Errorf("secret = %q, want override", cfg.Auth.Secret)
	}
	if cfg.Auth.AccessTokenTTL != time.Minute {
		t.Errorf("access ttl = %v, want %v", cfg.Auth.AccessTokenTTL, time.Minute)
	}
	if cfg.Auth.RefreshTokenTTL != 24*time.Hour {
		t.Errorf("refresh ttl = %v, want %v", cfg.Auth.RefreshTokenTTL, 24*time.Hour)
	}
}

func TestValidateRequiresAuthSecret(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{name: "missing in development", cfg: Config{Environment: "development"}},
		{name: "missing in production", cfg: Config{Environment: "production"}},
		{name: "empty in development", cfg: Config{Environment: "development", Auth: AuthConfig{Secret: ""}}},
		{name: "empty in production", cfg: Config{Environment: "production", Auth: AuthConfig{Secret: ""}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Error("Validate() error = nil, want missing-secret rejection")
			}
		})
	}
}

func TestValidateRejectsWeakSecretOutsideDevelopment(t *testing.T) {
	if err := (Config{Environment: "development", Auth: AuthConfig{Secret: "short"}}).Validate(); err != nil {
		t.Errorf("development Validate() error = %v, want nil", err)
	}

	if err := (Config{Environment: "production", Auth: AuthConfig{Secret: "short"}}).Validate(); err == nil {
		t.Error("production Validate() error = nil, want rejection")
	}

	strong := "a-strong-secret-value-that-is-long-enough"
	if err := (Config{Environment: "production", Auth: AuthConfig{Secret: strong}}).Validate(); err != nil {
		t.Errorf("production Validate() error = %v, want nil", err)
	}
}

func TestLoadCORSOrigins(t *testing.T) {
	if value, ok := os.LookupEnv("APP_CORS_ALLOWED_ORIGINS"); ok {
		t.Cleanup(func() { os.Setenv("APP_CORS_ALLOWED_ORIGINS", value) })
		os.Unsetenv("APP_CORS_ALLOWED_ORIGINS")
	}

	got := Load().CORS.AllowedOrigins
	want := []string{"http://localhost:3000", "http://localhost:5173"}
	if len(got) != len(want) {
		t.Fatalf("origins = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origin[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	t.Setenv("APP_CORS_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")
	override := Load().CORS.AllowedOrigins
	if len(override) != 2 || override[0] != "https://app.example.com" || override[1] != "https://admin.example.com" {
		t.Errorf("origins = %v, want parsed override", override)
	}
}

func TestLoadRateLimitDefaults(t *testing.T) {
	for _, key := range []string{
		"APP_RATE_LIMIT_LOGIN_PER_MINUTE",
		"APP_RATE_LIMIT_REFRESH_PER_MINUTE",
		"APP_RATE_LIMIT_GENERAL_PER_MINUTE",
	} {
		if value, ok := os.LookupEnv(key); ok {
			t.Cleanup(func() { os.Setenv(key, value) })
			os.Unsetenv(key)
		}
	}

	cfg := Load().RateLimit

	if cfg.LoginPerMinute != defaultLoginRateLimit {
		t.Errorf("login limit = %d, want %d", cfg.LoginPerMinute, defaultLoginRateLimit)
	}
	if cfg.RefreshPerMinute != defaultRefreshRateLimit {
		t.Errorf("refresh limit = %d, want %d", cfg.RefreshPerMinute, defaultRefreshRateLimit)
	}
	if cfg.GeneralPerMinute != defaultGeneralRateLimit {
		t.Errorf("general limit = %d, want %d", cfg.GeneralPerMinute, defaultGeneralRateLimit)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
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
