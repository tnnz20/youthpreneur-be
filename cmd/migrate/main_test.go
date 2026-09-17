package main

import (
	"strings"
	"testing"
)

func TestMigrationDSNRewritesScheme(t *testing.T) {
	got, err := migrationDSN("postgres://user:pass@localhost:5432/youthpreneur?sslmode=disable")
	if err != nil {
		t.Fatalf("migrationDSN() error = %v", err)
	}

	want := "pgx5://user:pass@localhost:5432/youthpreneur?sslmode=disable"
	if got != want {
		t.Errorf("migrationDSN() = %q, want %q", got, want)
	}
}

func TestMigrationDSNReturnsParseError(t *testing.T) {
	_, err := migrationDSN("postgres://user:p%zz@localhost:5432/youthpreneur")
	if err == nil {
		t.Fatal("migrationDSN() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parse postgres dsn") {
		t.Errorf("migrationDSN() error = %q, want wrapped parse error", err)
	}
}
