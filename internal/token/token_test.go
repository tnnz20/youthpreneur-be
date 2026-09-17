package token_test

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/token"
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func testUser() entity.User {
	return entity.User{
		ID:       42,
		PublicID: "YTP-000042",
		Email:    "alice@example.com",
		Role:     entity.RoleMember,
		IsActive: true,
	}
}

func TestIssueAccessRoundTrip(t *testing.T) {
	service := token.NewService("test-secret", 15*time.Minute)
	now := time.Now()

	raw, err := service.IssueAccess(testUser(), now)
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}

	claims, err := service.ParseAccess(raw)
	if err != nil {
		t.Fatalf("ParseAccess() error = %v", err)
	}
	if claims.UserID != 42 || claims.PublicID != "YTP-000042" || claims.Role != entity.RoleMember {
		t.Errorf("claims = %+v, want user id, public id, and role", claims)
	}
	if claims.Issuer != "youthpreneur-be" {
		t.Errorf("issuer = %q, want youthpreneur-be", claims.Issuer)
	}
	if claims.ExpiresAt.Unix() != now.Add(15*time.Minute).Unix() {
		t.Errorf("expiry = %v, want %v", claims.ExpiresAt.Time, now.Add(15*time.Minute))
	}
}

func TestParseAccessRejectsExpiredToken(t *testing.T) {
	service := token.NewService("test-secret", 15*time.Minute)

	raw, err := service.IssueAccess(testUser(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}

	if _, err := service.ParseAccess(raw); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("ParseAccess() error = %v, want ErrInvalidToken", err)
	}
}

func TestParseAccessRejectsWrongSecret(t *testing.T) {
	issuer := token.NewService("test-secret", 15*time.Minute)
	raw, err := issuer.IssueAccess(testUser(), time.Now())
	if err != nil {
		t.Fatalf("IssueAccess() error = %v", err)
	}

	other := token.NewService("another-secret", 15*time.Minute)
	if _, err := other.ParseAccess(raw); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("ParseAccess() error = %v, want ErrInvalidToken", err)
	}
}

func TestParseAccessRejectsGarbage(t *testing.T) {
	service := token.NewService("test-secret", 15*time.Minute)

	if _, err := service.ParseAccess("not-a-jwt"); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("ParseAccess() error = %v, want ErrInvalidToken", err)
	}
}

func TestGenerateRefreshIsRandomAndHashIsStable(t *testing.T) {
	service := token.NewService("test-secret", 15*time.Minute)

	first, err := service.GenerateRefresh()
	if err != nil {
		t.Fatalf("GenerateRefresh() error = %v", err)
	}
	second, err := service.GenerateRefresh()
	if err != nil {
		t.Fatalf("GenerateRefresh() error = %v", err)
	}
	if first == "" || second == "" || first == second {
		t.Errorf("refresh tokens = (%q, %q), want two distinct non-empty values", first, second)
	}

	hash := service.HashRefresh(first)
	if !hexPattern.MatchString(hash) {
		t.Errorf("hash = %q, want 64 lowercase hex characters", hash)
	}
	if hash != service.HashRefresh(first) {
		t.Error("HashRefresh() is not deterministic")
	}
	if hash == service.HashRefresh(second) {
		t.Error("distinct refresh tokens produced the same hash")
	}
}
