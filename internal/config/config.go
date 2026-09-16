package config

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config contains application and PostgreSQL settings loaded from the environment.
type Config struct {
	Addr            string
	LogLevel        string
	Environment     string
	Version         string
	ShutdownTimeout time.Duration
	Postgres        PostgresConfig
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

const defaultShutdownTimeout = 10 * time.Second

// Load reads application settings from environment variables and defaults.
func Load() Config {
	v := viper.New()

	v.SetDefault("app.addr", ":8080")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.version", "dev")
	v.SetDefault("app.shutdown_timeout", defaultShutdownTimeout)
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
	v.MustBindEnv("app.environment", "APP_ENVIRONMENT")
	v.MustBindEnv("app.version", "APP_VERSION")
	v.MustBindEnv("app.shutdown_timeout", "APP_SHUTDOWN_TIMEOUT")
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

	return Config{
		Addr:            v.GetString("app.addr"),
		LogLevel:        v.GetString("app.log_level"),
		Environment:     v.GetString("app.environment"),
		Version:         v.GetString("app.version"),
		ShutdownTimeout: shutdownTimeout,
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
