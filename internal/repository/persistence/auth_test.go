package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

// TestRefreshSessionRepositoryIntegration exercises refresh session SQL against
// PostgreSQL. It runs only when TEST_POSTGRES_DSN points at a database with
// migrations applied.
func TestRefreshSessionRepositoryIntegration(t *testing.T) {
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

	suffix := time.Now().UnixNano()
	now := time.Now().Unix()
	publicID := fmt.Sprintf("YTP-%06d", suffix%1000000)
	email := fmt.Sprintf("session-%d@example.com", suffix)

	users := NewUserRepository(db)
	user, err := users.CreateUser(ctx, entity.User{
		PublicID:  publicID,
		Email:     email,
		Password:  "hash",
		Role:      entity.RoleMember,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
		Profile:   &entity.Profile{FullName: "Session"},
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM users WHERE public_id = $1", publicID) })

	repo := NewRefreshSessionRepository(db)
	oldHash := fmt.Sprintf("old-%d", suffix)
	newHash := fmt.Sprintf("new-%d", suffix)

	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: oldHash,
		ExpiresAt: now + 3600,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateRefreshSession() error = %v", err)
	}

	found, err := repo.FindRefreshSession(ctx, oldHash)
	if err != nil {
		t.Fatalf("FindRefreshSession() error = %v", err)
	}
	if found.UserID != user.ID || found.ExpiresAt != now+3600 {
		t.Errorf("FindRefreshSession() = %+v, want persisted session", found)
	}

	consumed, err := repo.RotateRefreshSession(ctx, oldHash, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: newHash,
		ExpiresAt: now + 7200,
		CreatedAt: now + 1,
	}, now+1)
	if err != nil {
		t.Fatalf("RotateRefreshSession() error = %v", err)
	}
	if consumed.UserID != user.ID || consumed.ReplacedByHash != newHash {
		t.Errorf("RotateRefreshSession() = %+v, want consumed session with replacement", consumed)
	}
	if consumed.RevokedAt == nil || *consumed.RevokedAt != now+1 {
		t.Errorf("revoked at = %v, want %d", consumed.RevokedAt, now+1)
	}

	if _, err := repo.FindRefreshSession(ctx, newHash); err != nil {
		t.Fatalf("FindRefreshSession(new) error = %v", err)
	}
	revoked, err := repo.FindRefreshSession(ctx, oldHash)
	if err != nil {
		t.Fatalf("FindRefreshSession(old) error = %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Error("old session revoked_at = nil, want revoke timestamp")
	}

	if _, err := repo.RotateRefreshSession(ctx, oldHash, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: fmt.Sprintf("reuse-%d", suffix),
		ExpiresAt: now + 7200,
		CreatedAt: now + 2,
	}, now+2); !errors.Is(err, repository.ErrRefreshSessionNotFound) {
		t.Errorf("reused rotate error = %v, want ErrRefreshSessionNotFound", err)
	}

	if _, err := repo.FindRefreshSession(ctx, fmt.Sprintf("missing-%d", suffix)); !errors.Is(err, repository.ErrRefreshSessionNotFound) {
		t.Errorf("missing find error = %v, want ErrRefreshSessionNotFound", err)
	}

	if err := repo.RevokeRefreshSession(ctx, newHash, now+3); err != nil {
		t.Fatalf("RevokeRefreshSession() error = %v", err)
	}
	if err := repo.RevokeRefreshSession(ctx, newHash, now+4); err != nil {
		t.Fatalf("second RevokeRefreshSession() error = %v", err)
	}

	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: fmt.Sprintf("extra-%d", suffix),
		ExpiresAt: now + 3600,
		CreatedAt: now + 5,
	}); err != nil {
		t.Fatalf("CreateRefreshSession(extra) error = %v", err)
	}
	if err := repo.RevokeUserRefreshSessions(ctx, user.ID, now+6); err != nil {
		t.Fatalf("RevokeUserRefreshSessions() error = %v", err)
	}

	activeHash := fmt.Sprintf("active-%d", suffix)
	expiredHash := fmt.Sprintf("expired-%d", suffix)
	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: activeHash,
		ExpiresAt: now + 3600,
		CreatedAt: now + 7,
	}); err != nil {
		t.Fatalf("CreateRefreshSession(active) error = %v", err)
	}
	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: expiredHash,
		ExpiresAt: now - 1,
		CreatedAt: now - 3600,
	}); err != nil {
		t.Fatalf("CreateRefreshSession(expired) error = %v", err)
	}

	revokedHash := fmt.Sprintf("revoked-%d", suffix)
	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		TokenHash: revokedHash,
		ExpiresAt: now + 3600,
		CreatedAt: now + 8,
	}); err != nil {
		t.Fatalf("CreateRefreshSession(revoked) error = %v", err)
	}
	if err := repo.RevokeRefreshSession(ctx, revokedHash, now+8); err != nil {
		t.Fatalf("RevokeRefreshSession(revoked) error = %v", err)
	}

	if err := repo.DeleteExpiredRefreshSessions(ctx, now); err != nil {
		t.Fatalf("DeleteExpiredRefreshSessions() error = %v", err)
	}
	if _, err := repo.FindRefreshSession(ctx, expiredHash); !errors.Is(err, repository.ErrRefreshSessionNotFound) {
		t.Errorf("expired session find error = %v, want ErrRefreshSessionNotFound after cleanup", err)
	}
	if _, err := repo.FindRefreshSession(ctx, activeHash); err != nil {
		t.Errorf("active session find error = %v, want retained", err)
	}
	retained, err := repo.FindRefreshSession(ctx, revokedHash)
	if err != nil {
		t.Errorf("revoked unexpired session find error = %v, want retained for replay detection", err)
	} else if retained.RevokedAt == nil {
		t.Error("revoked unexpired session lost revoked_at during cleanup")
	}
}
