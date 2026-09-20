package persistence

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
	"github.com/tnnz20/youthpreneur-be/internal/repository"
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
			want: repository.ErrDuplicateEmail,
		},
		{
			name: "duplicate public id",
			err:  &pgconn.PgError{Code: uniqueViolation, ConstraintName: "users_public_id_key"},
			want: repository.ErrDuplicatePublicID,
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

	if errors.Is(err, repository.ErrDuplicateEmail) || errors.Is(err, repository.ErrDuplicatePublicID) {
		t.Fatalf("mapInsertError() = %v, want wrapped generic error", err)
	}
	if !strings.Contains(err.Error(), "connection reset") {
		t.Errorf("mapInsertError() = %v, want message to include cause", err)
	}
}

func TestFindUsersQueryExcludesAdminsBeforePagination(t *testing.T) {
	roleAt := strings.Index(findUsersQuery, "u.role <> $4")
	cursorAt := strings.Index(findUsersQuery, "$3::int = 0 OR u.id > $3")
	limitAt := strings.Index(findUsersQuery, "LIMIT $5")

	if roleAt == -1 || cursorAt == -1 || limitAt == -1 {
		t.Fatalf("findUsersQuery missing admin exclusion, cursor, or limit clause")
	}
	if !(roleAt < cursorAt && cursorAt < limitAt) {
		t.Errorf("admin exclusion must precede cursor and limit, got role=%d cursor=%d limit=%d", roleAt, cursorAt, limitAt)
	}
}

func TestFindUsersQuerySearchesFullNameCaseInsensitively(t *testing.T) {
	if !strings.Contains(findUsersQuery, "p.full_name ILIKE '%' || $6 || '%'") {
		t.Errorf("findUsersQuery missing parameterized case-insensitive full_name search")
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

	if err := repo.ChangePassword(ctx, publicID, "stale-hash", "ignored", now+3); !errors.Is(err, repository.ErrUserNotFound) {
		t.Errorf("ChangePassword() with stale hash error = %v, want ErrUserNotFound", err)
	}
	if err := repo.ChangePassword(ctx, publicID, "new-hash", "newer-hash", now+3); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
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

	if _, err := repo.FindUserByPublicID(ctx, publicID); !errors.Is(err, repository.ErrUserNotFound) {
		t.Errorf("FindUserByPublicID() error = %v, want ErrUserNotFound", err)
	}
	if err := repo.SoftDeleteUser(ctx, publicID, now+5); !errors.Is(err, repository.ErrUserNotFound) {
		t.Errorf("second SoftDeleteUser() error = %v, want ErrUserNotFound", err)
	}
}

// TestUserRepositoryListExcludesAdminsIntegration proves GET /users backing SQL
// omits admins while direct public-ID lookup still returns them. It runs only
// when TEST_POSTGRES_DSN points at a database with migrations applied.
func TestUserRepositoryListExcludesAdminsIntegration(t *testing.T) {
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
	district := fmt.Sprintf("ListFilter-%d", suffix)
	now := time.Now().Unix()

	members := make([]entity.User, 0, 2)
	for i := range 2 {
		member, err := repo.CreateUser(ctx, entity.User{
			PublicID:  fmt.Sprintf("YTP-%06d", (suffix+int64(i))%1000000),
			Email:     fmt.Sprintf("list-member-%d-%d@example.com", suffix, i),
			Password:  "hash",
			Role:      entity.RoleMember,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
			Profile:   &entity.Profile{FullName: "List Member", District: district},
		})
		if err != nil {
			t.Fatalf("CreateUser(member) error = %v", err)
		}
		members = append(members, member)
		t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE public_id = $1", member.PublicID) })
	}

	admin, err := repo.CreateUser(ctx, entity.User{
		PublicID:  fmt.Sprintf("YTP-%06d", (suffix+100)%1000000),
		Email:     fmt.Sprintf("list-admin-%d@example.com", suffix),
		Password:  "hash",
		Role:      entity.RoleAdmin,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
		Profile:   &entity.Profile{FullName: "List Admin", District: district},
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE public_id = $1", admin.PublicID) })

	first, err := repo.FindUsers(ctx, entity.UserFilter{District: district, Limit: 1})
	if err != nil {
		t.Fatalf("FindUsers() error = %v", err)
	}
	if len(first) != 1 || first[0].ID != members[0].ID {
		t.Fatalf("first page = %+v, want member %d", first, members[0].ID)
	}
	if containsPublicID(first, admin.PublicID) {
		t.Error("FindUsers() returned an admin account")
	}

	second, err := repo.FindUsers(ctx, entity.UserFilter{District: district, Cursor: first[0].ID, Limit: 1})
	if err != nil {
		t.Fatalf("FindUsers(cursor) error = %v", err)
	}
	if len(second) != 1 || second[0].ID != members[1].ID {
		t.Fatalf("second page = %+v, want member %d", second, members[1].ID)
	}
	if containsPublicID(second, admin.PublicID) {
		t.Error("FindUsers(cursor) returned an admin account")
	}

	last, err := repo.FindUsers(ctx, entity.UserFilter{District: district, Cursor: second[0].ID, Limit: 1})
	if err != nil {
		t.Fatalf("FindUsers(last) error = %v", err)
	}
	if len(last) != 0 {
		t.Fatalf("last page = %+v, want empty after members", last)
	}

	direct, err := repo.FindUserByPublicID(ctx, admin.PublicID)
	if err != nil {
		t.Fatalf("FindUserByPublicID(admin) error = %v", err)
	}
	if direct.Role != entity.RoleAdmin || direct.ID != admin.ID {
		t.Errorf("FindUserByPublicID(admin) = %+v, want active admin %d", direct, admin.ID)
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
