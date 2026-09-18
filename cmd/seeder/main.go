// Command seeder provisions the initial admin account from environment
// variables. It is a trusted provisioning path: credentials come from the
// process environment, never from request input.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/config"
	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
	"github.com/tnnz20/youthpreneur-be/internal/repository/persistence"
	"github.com/tnnz20/youthpreneur-be/internal/usecase"
)

const (
	envAdminEmail    = "SEEDER_ADMIN_EMAIL"
	envAdminPassword = "SEEDER_ADMIN_PASSWORD"
	// publicIDAttempts bounds retries when a generated public id collides.
	publicIDAttempts = 5
)

// newAdminProfile returns the seeded admin profile defaults. Only full_name is
// set; the optional profile fields stay empty, which the model accepts. These
// are seed defaults, never request input, so they are built fresh per call
// rather than shared through mutable package state.
func newAdminProfile() entity.Profile {
	return entity.Profile{FullName: "System Administrator"}
}

// adminCreator is the subset of repository.UserRepository the seeder needs.
// CreateUser inserts the user and profile in one transaction and maps unique
// violations to repository sentinel errors.
type adminCreator interface {
	CreateUser(ctx context.Context, user entity.User) (entity.User, error)
}

func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout, time.Now); err != nil {
		fmt.Fprintln(os.Stderr, "seeder:", err)
		os.Exit(1)
	}
}

// run validates the seed credentials before opening the database, then creates
// the admin. It returns an error and writes nothing to out on failure.
func run(ctx context.Context, getenv func(string) string, out io.Writer, now func() time.Time) error {
	email := usecase.NormalizeEmail(getenv(envAdminEmail))
	password := getenv(envAdminPassword)

	if email == "" {
		return fmt.Errorf("%s is required", envAdminEmail)
	}
	if password == "" {
		return fmt.Errorf("%s is required", envAdminPassword)
	}
	if err := usecase.ValidateEmail(email); err != nil {
		return fmt.Errorf("%s: %w", envAdminEmail, err)
	}
	if err := usecase.ValidatePassword(password); err != nil {
		return fmt.Errorf("%s: %w", envAdminPassword, err)
	}

	cfg := config.Load()

	db, err := config.OpenPostgres(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	return seedAdmin(ctx, persistence.NewUserRepository(db), email, password, now, out)
}

// seedAdmin hashes the password and inserts one active admin user with its
// profile. Colliding public ids are retried; a duplicate email fails without a
// partial write. Success logs only the email and public id.
func seedAdmin(
	ctx context.Context,
	repo adminCreator,
	email string,
	password string,
	now func() time.Time,
	out io.Writer,
) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	timestamp := now().Unix()

	for range publicIDAttempts {
		publicID, err := usecase.GeneratePublicID()
		if err != nil {
			return fmt.Errorf("generate public id: %w", err)
		}

		profile := newAdminProfile()
		user, err := repo.CreateUser(ctx, entity.User{
			PublicID:  publicID,
			Email:     email,
			Password:  string(passwordHash),
			Role:      entity.RoleAdmin,
			IsActive:  true,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
			Profile:   &profile,
		})
		switch {
		case errors.Is(err, repository.ErrDuplicatePublicID):
			continue
		case errors.Is(err, repository.ErrDuplicateEmail):
			return fmt.Errorf("%s is already registered", email)
		case err != nil:
			return fmt.Errorf("create admin user: %w", err)
		}

		fmt.Fprintf(out, "seeder: admin ready email=%s public_id=%s\n", user.Email, user.PublicID)

		return nil
	}

	return errors.New("allocate admin public id: attempts exhausted")
}
