package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// EnvironmentProduction is the environment value that enables production-only
// security behavior such as secure cookies.
const EnvironmentProduction = "production"

// minAuthSecretLength is the minimum accepted auth signing secret outside
// development, in bytes.
const minAuthSecretLength = 32

// Config contains application and PostgreSQL settings loaded from the environment.
type Config struct {
	Addr            string
	LogLevel        string
	Environment     string
	Version         string
	ShutdownTimeout time.Duration
	SecureCookies   bool
	Postgres        PostgresConfig
	Auth            AuthConfig
	CORS            CORSConfig
	RateLimit       RateLimitConfig
}

// PostgresConfig contains PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// AuthConfig contains JWT signing and token lifetime settings.
type AuthConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// CORSConfig contains the browser origins allowed to call the API with
// credentials.
type CORSConfig struct {
	AllowedOrigins []string
}

// RateLimitConfig contains per-minute request limits keyed by client IP and
// endpoint bucket.
type RateLimitConfig struct {
	LoginPerMinute   int
	RefreshPerMinute int
	GeneralPerMinute int
}

// Validate rejects a missing auth secret in every environment and a weak secret
// outside development.
func (c Config) Validate() error {
	if c.Auth.Secret == "" {
		return errors.New("APP_AUTH_SECRET is required")
	}
	if c.Environment != "development" && len(c.Auth.Secret) < minAuthSecretLength {
		return fmt.Errorf("APP_AUTH_SECRET must be at least %d bytes outside development", minAuthSecretLength)
	}

	return nil
}

// DSN returns a PostgreSQL connection URL usable by database/sql clients.
// Credentials and the database name are escaped, so passwords may contain
// reserved URL characters.
func (c PostgresConfig) DSN() string {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:   c.Database,
	}

	query := dsn.Query()
	query.Set("sslmode", c.SSLMode)
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

const (
	defaultShutdownTimeout = 10 * time.Second
	defaultAccessTTL       = 15 * time.Minute
	defaultRefreshTTL      = 7 * 24 * time.Hour
	// defaultCORSOrigins covers the common local frontend ports.
	defaultCORSOrigins      = "http://localhost:3000,http://localhost:5173"
	defaultLoginRateLimit   = 5
	defaultRefreshRateLimit = 10
	defaultGeneralRateLimit = 60
)

// Load reads application settings from environment variables and defaults.
func Load() Config {
	v := viper.New()

	v.SetDefault("app.addr", ":8080")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.version", "dev")
	v.SetDefault("app.shutdown_timeout", defaultShutdownTimeout)
	v.SetDefault("auth.access_token_ttl", defaultAccessTTL)
	v.SetDefault("auth.refresh_token_ttl", defaultRefreshTTL)
	v.SetDefault("cors.allowed_origins", defaultCORSOrigins)
	v.SetDefault("rate_limit.login_per_minute", defaultLoginRateLimit)
	v.SetDefault("rate_limit.refresh_per_minute", defaultRefreshRateLimit)
	v.SetDefault("rate_limit.general_per_minute", defaultGeneralRateLimit)
	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.user", "postgres")
	v.SetDefault("postgres.password", "postgres")
	v.SetDefault("postgres.database", "youthpreneur")
	v.SetDefault("postgres.sslmode", "disable")

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.MustBindEnv("app.addr", "APP_ADDR")
	v.MustBindEnv("app.log_level", "APP_LOG_LEVEL")
	v.MustBindEnv("app.environment", "APP_ENV")
	v.MustBindEnv("app.version", "APP_VERSION")
	v.MustBindEnv("app.shutdown_timeout", "APP_SHUTDOWN_TIMEOUT")
	v.MustBindEnv("auth.secret", "APP_AUTH_SECRET")
	v.MustBindEnv("auth.access_token_ttl", "APP_AUTH_ACCESS_TOKEN_TTL")
	v.MustBindEnv("auth.refresh_token_ttl", "APP_AUTH_REFRESH_TOKEN_TTL")
	v.MustBindEnv("cors.allowed_origins", "APP_CORS_ALLOWED_ORIGINS")
	v.MustBindEnv("rate_limit.login_per_minute", "APP_RATE_LIMIT_LOGIN_PER_MINUTE")
	v.MustBindEnv("rate_limit.refresh_per_minute", "APP_RATE_LIMIT_REFRESH_PER_MINUTE")
	v.MustBindEnv("rate_limit.general_per_minute", "APP_RATE_LIMIT_GENERAL_PER_MINUTE")
	v.MustBindEnv("postgres.host", "POSTGRES_HOST")
	v.MustBindEnv("postgres.port", "POSTGRES_PORT")
	v.MustBindEnv("postgres.user", "POSTGRES_USER")
	v.MustBindEnv("postgres.password", "POSTGRES_PASSWORD")
	v.MustBindEnv("postgres.database", "POSTGRES_DB")
	v.MustBindEnv("postgres.sslmode", "POSTGRES_SSLMODE")

	shutdownTimeout := v.GetDuration("app.shutdown_timeout")
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	environment := v.GetString("app.environment")
	origins := splitOrigins(v.GetString("cors.allowed_origins"))

	return Config{
		Addr:            v.GetString("app.addr"),
		LogLevel:        v.GetString("app.log_level"),
		Environment:     environment,
		Version:         v.GetString("app.version"),
		ShutdownTimeout: shutdownTimeout,
		SecureCookies:   environment == EnvironmentProduction,
		Auth: AuthConfig{
			Secret:          v.GetString("auth.secret"),
			AccessTokenTTL:  v.GetDuration("auth.access_token_ttl"),
			RefreshTokenTTL: v.GetDuration("auth.refresh_token_ttl"),
		},
		CORS: CORSConfig{AllowedOrigins: origins},
		RateLimit: RateLimitConfig{
			LoginPerMinute:   v.GetInt("rate_limit.login_per_minute"),
			RefreshPerMinute: v.GetInt("rate_limit.refresh_per_minute"),
			GeneralPerMinute: v.GetInt("rate_limit.general_per_minute"),
		},
		Postgres: PostgresConfig{
			Host:     v.GetString("postgres.host"),
			Port:     v.GetInt("postgres.port"),
			User:     v.GetString("postgres.user"),
			Password: v.GetString("postgres.password"),
			Database: v.GetString("postgres.database"),
			SSLMode:  v.GetString("postgres.sslmode"),
		},
	}
}

// splitOrigins parses a comma-separated origin list, trimming blanks. An empty
// result falls back to the local development defaults.
func splitOrigins(raw string) []string {
	origins := make([]string, 0)
	for _, origin := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		for _, origin := range strings.Split(defaultCORSOrigins, ",") {
			origins = append(origins, strings.TrimSpace(origin))
		}
	}

	return origins
}
