package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config contains application and PostgreSQL settings loaded from the environment.
type Config struct {
	Addr     string
	LogLevel string
	Postgres PostgresConfig
}

// PostgresConfig contains PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// Load reads application settings from environment variables and defaults.
func Load() Config {
	v := viper.New()

	v.SetDefault("app.addr", ":8080")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.user", "postgres")
	v.SetDefault("postgres.password", "postgres")
	v.SetDefault("postgres.database", "youthpreneur")

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.MustBindEnv("app.addr", "APP_ADDR")
	v.MustBindEnv("app.log_level", "APP_LOG_LEVEL")
	v.MustBindEnv("postgres.host", "POSTGRES_HOST")
	v.MustBindEnv("postgres.port", "POSTGRES_PORT")
	v.MustBindEnv("postgres.user", "POSTGRES_USER")
	v.MustBindEnv("postgres.password", "POSTGRES_PASSWORD")
	v.MustBindEnv("postgres.database", "POSTGRES_DB")

	return Config{
		Addr:     v.GetString("app.addr"),
		LogLevel: v.GetString("app.log_level"),
		Postgres: PostgresConfig{
			Host:     v.GetString("postgres.host"),
			Port:     v.GetInt("postgres.port"),
			User:     v.GetString("postgres.user"),
			Password: v.GetString("postgres.password"),
			Database: v.GetString("postgres.database"),
		},
	}
}
