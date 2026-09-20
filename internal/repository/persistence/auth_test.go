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
	familyID := fmt.Sprintf("11111111-1111-4111-8111-%012d", suffix%1000000000000)
	otherFamilyID := fmt.Sprintf("22222222-2222-4222-8222-%012d", suffix%1000000000000)
	graceUntil := now + 10

	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		FamilyID:  familyID,
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
	if found.FamilyID != familyID {
		t.Errorf("family id = %q, want %q", found.FamilyID, familyID)
	}

	replacementEnc := []byte{0x01, 0x02, 0x03, 0x04}
	consumed, err := repo.RotateRefreshSession(ctx, oldHash, entity.RefreshSession{
		UserID:              user.ID,
		FamilyID:            familyID,
		TokenHash:           newHash,
		ExpiresAt:           now + 7200,
		CreatedAt:           now + 1,
		GraceUntil:          &graceUntil,
		ReplacementTokenEnc: replacementEnc,
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
	if consumed.RevocationReason != entity.ReasonRotated {
		t.Errorf("revocation reason = %q, want %q", consumed.RevocationReason, entity.ReasonRotated)
	}
	if consumed.GraceUntil == nil || *consumed.GraceUntil != graceUntil {
		t.Errorf("grace until = %v, want %d", consumed.GraceUntil, graceUntil)
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
	if revoked.RevocationReason != entity.ReasonRotated {
		t.Errorf("persisted revocation reason = %q, want %q", revoked.RevocationReason, entity.ReasonRotated)
	}
	if revoked.GraceUntil == nil || *revoked.GraceUntil != graceUntil {
		t.Errorf("persisted grace until = %v, want %d", revoked.GraceUntil, graceUntil)
	}
	if string(revoked.ReplacementTokenEnc) != string(replacementEnc) {
		t.Errorf("persisted replacement = %v, want %v", revoked.ReplacementTokenEnc, replacementEnc)
	}

	// A second family in the same user must survive a family-scoped revoke.
	otherHash := fmt.Sprintf("other-%d", suffix)
	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		FamilyID:  otherFamilyID,
		TokenHash: otherHash,
		ExpiresAt: now + 3600,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateRefreshSession(other) error = %v", err)
	}
	if err := repo.RevokeRefreshSessionFamily(ctx, familyID, entity.ReasonReplay, now+5); err != nil {
		t.Fatalf("RevokeRefreshSessionFamily() error = %v", err)
	}
	if kept, err := repo.FindRefreshSession(ctx, otherHash); err != nil || kept.RevokedAt != nil {
		t.Errorf("other family session = %+v (err %v), want untouched", kept, err)
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

	// Rollback: a rotate whose replacement hash already exists must fail and
	// leave the presented session active, proving the revoke and insert are one
	// transaction.
	rollbackOld := fmt.Sprintf("rollback-old-%d", suffix)
	rollbackGrace := now + 10
	if err := repo.CreateRefreshSession(ctx, entity.RefreshSession{
		UserID:    user.ID,
		FamilyID:  familyID,
		TokenHash: rollbackOld,
		ExpiresAt: now + 3600,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateRefreshSession(rollback) error = %v", err)
	}
	if _, err := repo.RotateRefreshSession(ctx, rollbackOld, entity.RefreshSession{
		UserID:     user.ID,
		FamilyID:   familyID,
		TokenHash:  newHash,
		ExpiresAt:  now + 7200,
		CreatedAt:  now + 1,
		GraceUntil: &rollbackGrace,
	}, now+1); err == nil {
		t.Fatal("RotateRefreshSession() error = nil, want duplicate replacement failure")
	}
	if survivor, err := repo.FindRefreshSession(ctx, rollbackOld); err != nil {
		t.Fatalf("FindRefreshSession(rollback) error = %v", err)
	} else if survivor.RevokedAt != nil {
		t.Error("failed rotate revoked the presented session instead of rolling back")
	}

	if err := repo.RevokeRefreshSession(ctx, newHash, entity.ReasonLogout, now+3); err != nil {
		t.Fatalf("RevokeRefreshSession() error = %v", err)
	}
	if err := repo.RevokeRefreshSession(ctx, newHash, entity.ReasonLogout, now+4); err != nil {
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
	if err := repo.RevokeUserRefreshSessions(ctx, user.ID, entity.ReasonPasswordChange, now+6); err != nil {
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
	if err := repo.RevokeRefreshSession(ctx, revokedHash, entity.ReasonLogout, now+8); err != nil {
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
