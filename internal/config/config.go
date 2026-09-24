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
	UploadDir       string
	Postgres        PostgresConfig
	Auth            AuthConfig
	CORS            CORSConfig
	RateLimit       RateLimitConfig
	Seeder          SeederConfig
}

// SeederConfig contains initial admin provisioning credentials.
type SeederConfig struct {
	AdminEmail    string
	AdminPassword string
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

// LoadSeederCredentials reads seed credentials from the required `.env` file.
func LoadSeederCredentials() SeederConfig {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config file: %w", err))
	}

	return SeederConfig{
		AdminEmail:    v.GetString("SEEDER_ADMIN_EMAIL"),
		AdminPassword: v.GetString("SEEDER_ADMIN_PASSWORD"),
	}
}

// Load reads application settings from the required `.env` file and defaults.
func Load() Config {
	v := viper.New()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			panic(fmt.Errorf("read config file: %w", err))
		}
	}

	v.SetDefault("APP_ADDR", ":8080")
	v.SetDefault("APP_LOG_LEVEL", "info")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_VERSION", "dev")
	v.SetDefault("APP_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	v.SetDefault("APP_AUTH_ACCESS_TOKEN_TTL", defaultAccessTTL)
	v.SetDefault("APP_AUTH_REFRESH_TOKEN_TTL", defaultRefreshTTL)
	v.SetDefault("APP_CORS_ALLOWED_ORIGINS", defaultCORSOrigins)
	v.SetDefault("APP_RATE_LIMIT_LOGIN_PER_MINUTE", defaultLoginRateLimit)
	v.SetDefault("APP_RATE_LIMIT_REFRESH_PER_MINUTE", defaultRefreshRateLimit)
	v.SetDefault("APP_RATE_LIMIT_GENERAL_PER_MINUTE", defaultGeneralRateLimit)
	v.SetDefault("UPLOAD_DIR", "./var/uploads")
	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_USER", "postgres")
	v.SetDefault("POSTGRES_PASSWORD", "postgres")
	v.SetDefault("POSTGRES_DB", "youthpreneur")
	v.SetDefault("POSTGRES_SSLMODE", "disable")

	v.AutomaticEnv()

	shutdownTimeout := v.GetDuration("APP_SHUTDOWN_TIMEOUT")
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}

	environment := v.GetString("APP_ENV")
	origins := splitOrigins(v.GetString("APP_CORS_ALLOWED_ORIGINS"))

	return Config{
		Addr:            v.GetString("APP_ADDR"),
		LogLevel:        v.GetString("APP_LOG_LEVEL"),
		Environment:     environment,
		Version:         v.GetString("APP_VERSION"),
		ShutdownTimeout: shutdownTimeout,
		SecureCookies:   environment == EnvironmentProduction,
		UploadDir:       v.GetString("UPLOAD_DIR"),
		Auth: AuthConfig{
			Secret:          v.GetString("APP_AUTH_SECRET"),
			AccessTokenTTL:  v.GetDuration("APP_AUTH_ACCESS_TOKEN_TTL"),
			RefreshTokenTTL: v.GetDuration("APP_AUTH_REFRESH_TOKEN_TTL"),
		},
		CORS: CORSConfig{AllowedOrigins: origins},
		RateLimit: RateLimitConfig{
			LoginPerMinute:   v.GetInt("APP_RATE_LIMIT_LOGIN_PER_MINUTE"),
			RefreshPerMinute: v.GetInt("APP_RATE_LIMIT_REFRESH_PER_MINUTE"),
			GeneralPerMinute: v.GetInt("APP_RATE_LIMIT_GENERAL_PER_MINUTE"),
		},
		Seeder: SeederConfig{
			AdminEmail:    v.GetString("SEEDER_ADMIN_EMAIL"),
			AdminPassword: v.GetString("SEEDER_ADMIN_PASSWORD"),
		},
		Postgres: PostgresConfig{
			Host:     v.GetString("POSTGRES_HOST"),
			Port:     v.GetInt("POSTGRES_PORT"),
			User:     v.GetString("POSTGRES_USER"),
			Password: v.GetString("POSTGRES_PASSWORD"),
			Database: v.GetString("POSTGRES_DB"),
			SSLMode:  v.GetString("POSTGRES_SSLMODE"),
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
