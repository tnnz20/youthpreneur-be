package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

var publicIDPattern = regexp.MustCompile(`^YTP-[0-9]{6}$`)

type fakeAdminCreator struct {
	calls        int
	failures     int
	err          error
	lastUser     entity.User
	lastPublicID string
}

func (f *fakeAdminCreator) CreateUser(_ context.Context, user entity.User) (entity.User, error) {
	f.calls++
	f.lastUser = user
	f.lastPublicID = user.PublicID
	if f.calls <= f.failures {
		return entity.User{}, repository.ErrDuplicatePublicID
	}
	if f.err != nil {
		return entity.User{}, f.err
	}

	created := user
	created.ID = f.calls

	return created, nil
}

func fixedNow() time.Time {
	return time.Unix(1_700_000_000, 0)
}

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestRunRequiresEmail(t *testing.T) {
	err := run(context.Background(), fakeGetenv(map[string]string{
		envAdminPassword: "password123",
	}), io.Discard, fixedNow)
	if err == nil || !strings.Contains(err.Error(), envAdminEmail) {
		t.Fatalf("run() error = %v, want %s required", err, envAdminEmail)
	}
}

func TestRunRequiresPassword(t *testing.T) {
	err := run(context.Background(), fakeGetenv(map[string]string{
		envAdminEmail: "admin@example.com",
	}), io.Discard, fixedNow)
	if err == nil || !strings.Contains(err.Error(), envAdminPassword) {
		t.Fatalf("run() error = %v, want %s required", err, envAdminPassword)
	}
}

func TestRunRejectsInvalidCredentials(t *testing.T) {
	cases := []struct {
		name  string
		email string
		pass  string
	}{
		{name: "invalid email", email: "not-an-email", pass: "password123"},
		{name: "short password", email: "admin@example.com", pass: "short"},
		{name: "password over bcrypt limit", email: "admin@example.com", pass: strings.Repeat("a", 73)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(context.Background(), fakeGetenv(map[string]string{
				envAdminEmail:    tc.email,
				envAdminPassword: tc.pass,
			}), io.Discard, fixedNow)
			if err == nil {
				t.Fatal("run() error = nil, want validation error")
			}
		})
	}
}

func TestSeedAdminCreatesActiveAdminWithDefaultProfile(t *testing.T) {
	repo := &fakeAdminCreator{}
	var out bytes.Buffer
	password := "password123"

	if err := seedAdmin(context.Background(), repo, "admin@example.com", password, fixedNow, &out); err != nil {
		t.Fatalf("seedAdmin() error = %v", err)
	}

	if repo.calls != 1 {
		t.Errorf("create calls = %d, want 1", repo.calls)
	}
	user := repo.lastUser
	if user.Role != entity.RoleAdmin {
		t.Errorf("role = %q, want %q", user.Role, entity.RoleAdmin)
	}
	if !user.IsActive {
		t.Error("is_active = false, want true")
	}
	if user.PublicID != repo.lastPublicID || !publicIDPattern.MatchString(user.PublicID) {
		t.Errorf("public id = %q, want YTP- plus six digits", user.PublicID)
	}
	if user.Email != "admin@example.com" {
		t.Errorf("email = %q, want admin@example.com", user.Email)
	}
	if user.CreatedAt != fixedNow().Unix() || user.UpdatedAt != user.CreatedAt {
		t.Errorf("timestamps = (%d, %d), want equal fixed epoch", user.CreatedAt, user.UpdatedAt)
	}
	if user.Profile == nil || user.Profile.FullName != adminProfile.FullName {
		t.Errorf("profile = %+v, want default full name %q", user.Profile, adminProfile.FullName)
	}
	if user.Password == password {
		t.Error("password was stored in plaintext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		t.Errorf("stored hash does not match password: %v", err)
	}

	logged := out.String()
	if !strings.Contains(logged, "admin@example.com") || !strings.Contains(logged, user.PublicID) {
		t.Errorf("success log = %q, want email and public id", logged)
	}
	if strings.Contains(logged, password) || strings.Contains(logged, user.Password) {
		t.Errorf("success log leaks password material: %q", logged)
	}
}

func TestSeedAdminMapsDuplicateEmail(t *testing.T) {
	repo := &fakeAdminCreator{err: repository.ErrDuplicateEmail}
	var out bytes.Buffer

	err := seedAdmin(context.Background(), repo, "admin@example.com", "password123", fixedNow, &out)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("seedAdmin() error = %v, want duplicate email error", err)
	}
	if out.Len() != 0 {
		t.Errorf("success was logged on duplicate email: %q", out.String())
	}
}

func TestSeedAdminWrapsTransactionFailure(t *testing.T) {
	repo := &fakeAdminCreator{err: errors.New("tx commit failed")}
	var out bytes.Buffer

	err := seedAdmin(context.Background(), repo, "admin@example.com", "password123", fixedNow, &out)
	if err == nil || !strings.Contains(err.Error(), "create admin user") || !strings.Contains(err.Error(), "tx commit failed") {
		t.Fatalf("seedAdmin() error = %v, want wrapped transaction failure", err)
	}
	if out.Len() != 0 {
		t.Errorf("success was logged on failure: %q", out.String())
	}
}

func TestSeedAdminRetriesPublicIDCollision(t *testing.T) {
	repo := &fakeAdminCreator{failures: 2}

	if err := seedAdmin(context.Background(), repo, "admin@example.com", "password123", fixedNow, io.Discard); err != nil {
		t.Fatalf("seedAdmin() error = %v", err)
	}
	if repo.calls != 3 {
		t.Errorf("create calls = %d, want 3", repo.calls)
	}
}

func TestSeedAdminStopsAfterPublicIDAttempts(t *testing.T) {
	repo := &fakeAdminCreator{failures: publicIDAttempts}

	err := seedAdmin(context.Background(), repo, "admin@example.com", "password123", fixedNow, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "attempts exhausted") {
		t.Fatalf("seedAdmin() error = %v, want attempts exhausted", err)
	}
	if repo.calls != publicIDAttempts {
		t.Errorf("create calls = %d, want %d", repo.calls, publicIDAttempts)
	}
}
