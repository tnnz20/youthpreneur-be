package config_test

import (
	"testing"

	"github.com/tnnz20/youthpreneur-be/internal/config"
)

func TestSSHConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.SSHConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: config.SSHConfig{
				Host: "192.168.1.1",
				Port: 22,
				User: "ubuntu",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: config.SSHConfig{
				Port: 22,
				User: "ubuntu",
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			cfg: config.SSHConfig{
				Host: "192.168.1.1",
				Port: 0,
				User: "ubuntu",
			},
			wantErr: true,
		},
		{
			name: "missing user",
			cfg: config.SSHConfig{
				Host: "192.168.1.1",
				Port: 22,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSSHPostgresConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.SSHPostgresConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: config.SSHPostgresConfig{
				Host:     "127.0.0.1",
				Port:     5432,
				Database: "youthpreneur",
				User:     "postgres",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: config.SSHPostgresConfig{
				Port:     5432,
				Database: "youthpreneur",
				User:     "postgres",
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			cfg: config.SSHPostgresConfig{
				Host:     "127.0.0.1",
				Port:     -1,
				Database: "youthpreneur",
				User:     "postgres",
			},
			wantErr: true,
		},
		{
			name: "missing database",
			cfg: config.SSHPostgresConfig{
				Host: "127.0.0.1",
				Port: 5432,
				User: "postgres",
			},
			wantErr: true,
		},
		{
			name: "missing user",
			cfg: config.SSHPostgresConfig{
				Host:     "127.0.0.1",
				Port:     5432,
				Database: "youthpreneur",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSSHPostgresConfigDSNFor(t *testing.T) {
	cfg := config.SSHPostgresConfig{
		Host:     "remote-db",
		Port:     5432,
		Database: "proddb",
		User:     "admin",
		Password: "secret/password!#",
		SSLMode:  "disable",
	}

	dsn := cfg.DSNFor("127.0.0.1", 61234)
	want := "postgres://admin:secret%2Fpassword%21%23@127.0.0.1:61234/proddb?sslmode=disable"
	if dsn != want {
		t.Errorf("DSNFor() = %q, want %q", dsn, want)
	}
}

func TestLoadSSHConfigFromEnv(t *testing.T) {
	t.Setenv("SSH_HOST", "ssh.example.com")
	t.Setenv("SSH_PORT", "2222")
	t.Setenv("SSH_USER", "deploy")
	t.Setenv("SSH_PASSWORD", "sshpass")
	t.Setenv("SSH_POSTGRES_HOST", "10.0.0.5")
	t.Setenv("SSH_POSTGRES_PORT", "5432")
	t.Setenv("SSH_POSTGRES_DATABASE", "appdb")
	t.Setenv("SSH_POSTGRES_USER", "dbuser")
	t.Setenv("SSH_POSTGRES_PASSWORD", "dbpass")

	sshCfg, pgCfg, err := config.LoadSSHConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sshCfg.Host != "ssh.example.com" || sshCfg.Port != 2222 || sshCfg.User != "deploy" {
		t.Errorf("unexpected sshCfg: %+v", sshCfg)
	}
	if pgCfg.Host != "10.0.0.5" || pgCfg.Database != "appdb" || pgCfg.User != "dbuser" {
		t.Errorf("unexpected pgCfg: %+v", pgCfg)
	}
}
