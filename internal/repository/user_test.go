package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

func TestMapInsertErrorTranslatesUniqueViolations(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "duplicate email",
			err:  &pgconn.PgError{Code: uniqueViolation, ConstraintName: "users_email_key"},
			want: ErrDuplicateEmail,
		},
		{
			name: "duplicate public id",
			err:  &pgconn.PgError{Code: uniqueViolation, ConstraintName: "users_public_id_key"},
			want: ErrDuplicatePublicID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapInsertError(tc.err); !errors.Is(got, tc.want) {
				t.Fatalf("mapInsertError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMapInsertErrorWrapsOtherFailures(t *testing.T) {
	err := mapInsertError(errors.New("connection reset"))

	if errors.Is(err, ErrDuplicateEmail) || errors.Is(err, ErrDuplicatePublicID) {
		t.Fatalf("mapInsertError() = %v, want wrapped generic error", err)
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("mapInsertError() = %v, want message to include cause", err)
	}
}

// TestUserRepositoryIntegration exercises real SQL against PostgreSQL. It runs
// only when TEST_POSTGRES_DSN points at a database with migrations applied.
func TestUserRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := NewUserRepository(db)
	suffix := time.Now().UnixNano()
	publicID := fmt.Sprintf("YTP-%06d", suffix%1000000)
	email := fmt.Sprintf("integration-%d@example.com", suffix)
	now := time.Now().Unix()
	birthDate := time.Date(1995, time.March, 14, 0, 0, 0, 0, time.UTC)

	created, err := repo.CreateUser(ctx, entity.User{
		PublicID:  publicID,
		Email:     email,
		Password:  "hash",
		Role:      entity.RoleMember,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
		Profile: &entity.Profile{
			FullName:  "Integration",
			NIK:       "3273010101010001",
			BirthDate: &birthDate,
			District:  "Bandung",
			Phone:     "08123456789",
			Address:   "Jalan Mawar 1",
		},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.ID == 0 {
		t.Fatal("CreateUser() returned zero id")
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE public_id = $1", publicID) })

	found, err := repo.FindUserByPublicID(ctx, publicID)
	if err != nil {
		t.Fatalf("FindUserByPublicID() error = %v", err)
	}
	if found.Email != email || found.Profile == nil ||
		found.Profile.District != "Bandung" || found.Profile.NIK != "3273010101010001" ||
		found.Profile.Phone != "08123456789" || found.Profile.Address != "Jalan Mawar 1" {
		t.Errorf("FindUserByPublicID() = %+v, want persisted user with profile", found)
	}
	if found.Profile.BirthDate == nil || !found.Profile.BirthDate.Equal(birthDate) {
		t.Errorf("birth date = %v, want %v", found.Profile.BirthDate, birthDate)
	}

	users, err := repo.FindUsers(ctx, entity.UserFilter{District: "Bandung", Limit: 100})
	if err != nil {
		t.Fatalf("FindUsers() error = %v", err)
	}
	if !containsPublicID(users, publicID) {
		t.Errorf("FindUsers() did not return %s", publicID)
	}

	updated, err := repo.UpdateStatus(ctx, publicID, false, now+1)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.IsActive {
		t.Error("UpdateStatus() left is_active true")
	}

	if _, err := repo.UpdateProfile(ctx, publicID, entity.Profile{FullName: "Renamed", Gender: entity.GenderFemale}, now+2); err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	if err := repo.UpdatePassword(ctx, publicID, "new-hash", now+3); err != nil {
		t.Fatalf("UpdatePassword() error = %v", err)
	}

	if err := repo.SoftDeleteUser(ctx, publicID, now+4); err != nil {
		t.Fatalf("SoftDeleteUser() error = %v", err)
	}

	var profileDeletedAt int64
	if err := db.QueryRowContext(ctx, `
		SELECT p.deleted_at
		FROM user_profiles p
		JOIN users u ON u.id = p.user_id
		WHERE u.public_id = $1`, publicID).Scan(&profileDeletedAt); err != nil {
		t.Fatalf("read profile deleted_at: %v", err)
	}
	if profileDeletedAt != now+4 {
		t.Errorf("profile deleted_at = %d, want %d", profileDeletedAt, now+4)
	}

	if _, err := repo.FindUserByPublicID(ctx, publicID); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("FindUserByPublicID() error = %v, want ErrUserNotFound", err)
	}
	if err := repo.SoftDeleteUser(ctx, publicID, now+5); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("second SoftDeleteUser() error = %v, want ErrUserNotFound", err)
	}
}

func containsPublicID(users []entity.User, publicID string) bool {
	for _, user := range users {
		if user.PublicID == publicID {
			return true
		}
	}

	return false
}
