package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/spf13/viper"
)

// SSHConfig holds connection details for the remote SSH bastion.
type SSHConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	KnownHostsFile string
}

// SSHPostgresConfig holds connection details for PostgreSQL accessed through the SSH tunnel.
type SSHPostgresConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
}

// DSNFor returns a PostgreSQL connection URL for the tunneled target with host and port rewritten.
func (c SSHPostgresConfig) DSNFor(localHost string, localPort int) string {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(localHost, strconv.Itoa(localPort)),
		Path:   c.Database,
	}

	query := dsn.Query()
	query.Set("sslmode", c.SSLMode)
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

// Validate checks that required SSH parameters are provided.
func (c SSHConfig) Validate() error {
	if c.Host == "" {
		return errors.New("SSH_HOST is required when SSH is enabled")
	}
	if c.Port <= 0 {
		return errors.New("SSH_PORT must be a positive integer")
	}
	if c.User == "" {
		return errors.New("SSH_USER is required when SSH is enabled")
	}

	return nil
}

// Validate checks that required SSH PostgreSQL parameters are provided.
func (c SSHPostgresConfig) Validate() error {
	if c.Host == "" {
		return errors.New("SSH_POSTGRES_HOST is required when SSH is enabled")
	}
	if c.Port <= 0 {
		return errors.New("SSH_POSTGRES_PORT must be a positive integer")
	}
	if c.Database == "" {
		return errors.New("SSH_POSTGRES_DATABASE is required when SSH is enabled")
	}
	if c.User == "" {
		return errors.New("SSH_POSTGRES_USER is required when SSH is enabled")
	}

	return nil
}

// LoadSSHConfig loads SSH and SSHPostgres configurations from .env and environment.
func LoadSSHConfig() (SSHConfig, SSHPostgresConfig, error) {
	v := viper.New()
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return SSHConfig{}, SSHPostgresConfig{}, fmt.Errorf("read config file: %w", err)
		}
	}

	v.SetDefault("SSH_PORT", 22)
	v.SetDefault("SSH_POSTGRES_HOST", "127.0.0.1")
	v.SetDefault("SSH_POSTGRES_PORT", 5432)
	v.SetDefault("SSH_POSTGRES_SSLMODE", "disable")

	v.AutomaticEnv()

	sshCfg := SSHConfig{
		Host:           v.GetString("SSH_HOST"),
		Port:           v.GetInt("SSH_PORT"),
		User:           v.GetString("SSH_USER"),
		Password:       v.GetString("SSH_PASSWORD"),
		KnownHostsFile: v.GetString("SSH_KNOWN_HOSTS_FILE"),
	}

	pgCfg := SSHPostgresConfig{
		Host:     v.GetString("SSH_POSTGRES_HOST"),
		Port:     v.GetInt("SSH_POSTGRES_PORT"),
		Database: v.GetString("SSH_POSTGRES_DATABASE"),
		User:     v.GetString("SSH_POSTGRES_USER"),
		Password: v.GetString("SSH_POSTGRES_PASSWORD"),
		SSLMode:  v.GetString("SSH_POSTGRES_SSLMODE"),
	}

	if err := sshCfg.Validate(); err != nil {
		return SSHConfig{}, SSHPostgresConfig{}, err
	}
	if err := pgCfg.Validate(); err != nil {
		return SSHConfig{}, SSHPostgresConfig{}, err
	}

	return sshCfg, pgCfg, nil
}
