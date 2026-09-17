// Command migrate applies, rolls back, and inspects database schema migrations.
package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/tnnz20/youthpreneur-be/db/migrations"
	"github.com/tnnz20/youthpreneur-be/internal/config"
)

const usage = "usage: migrate <up|down|force VERSION|version>"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}

	cfg := config.Load()

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	dsn, err := migrationDSN(cfg.Postgres.DSN())
	if err != nil {
		return fmt.Errorf("build migration dsn: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		return fmt.Errorf("open migration client: %w", err)
	}
	defer closeMigrator(m)

	switch args[0] {
	case "up":
		return apply(m.Up(), "up")
	case "down":
		return apply(m.Down(), "down")
	case "force":
		return force(m, args[1:])
	case "version":
		return printVersion(m)
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

// apply reports migration progress, treating ErrNoChange as success so repeated
// runs are safe.
func apply(err error, action string) error {
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Printf("migrate: %s skipped, no change\n", action)
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}

	fmt.Printf("migrate: %s complete\n", action)

	return nil
}

// force marks the migration version as applied or rolled back without running
// migrations, recovering a database left dirty by a failed migration.
func force(m *migrate.Migrate, args []string) error {
	if len(args) != 1 {
		return errors.New("force requires a version\n" + usage)
	}

	version, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("parse version %q: %w", args[0], err)
	}

	if err := m.Force(version); err != nil {
		return fmt.Errorf("force version %d: %w", version, err)
	}

	fmt.Printf("migrate: forced version %d\n", version)

	return nil
}

func printVersion(m *migrate.Migrate) error {
	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		fmt.Println("migrate: no migrations applied")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read version: %w", err)
	}

	state := "clean"
	if dirty {
		state = "dirty"
	}

	fmt.Printf("migrate: version %d (%s)\n", version, state)

	return nil
}

// migrationDSN rewrites the application DSN to the scheme registered by the
// golang-migrate pgx driver. That driver still connects through pgx. A DSN that
// cannot be parsed is reported instead of being passed through unchanged, since
// the raw postgres:// scheme is not registered with golang-migrate.
func migrationDSN(dsn string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse postgres dsn: %w", err)
	}

	parsed.Scheme = "pgx5"

	return parsed.String(), nil
}

func closeMigrator(m *migrate.Migrate) {
	sourceErr, databaseErr := m.Close()
	if sourceErr != nil {
		fmt.Fprintln(os.Stderr, "migrate: closing source:", sourceErr)
	}
	if databaseErr != nil {
		fmt.Fprintln(os.Stderr, "migrate: closing database:", databaseErr)
	}
}
