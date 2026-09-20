package repository

import (
	"context"
	"errors"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
)

// ErrRefreshSessionNotFound indicates no usable refresh session matches the
// presented token hash. It covers unknown, revoked, and expired sessions.
var ErrRefreshSessionNotFound = errors.New("repository: refresh session not found")

// RefreshSessionRepository persists opaque refresh token sessions.
//
// Callers must pass a non-nil context.
type RefreshSessionRepository interface {
	// CreateRefreshSession inserts a new active refresh session.
	CreateRefreshSession(ctx context.Context, session entity.RefreshSession) error
	// FindRefreshSession returns the session matching tokenHash, including
	// revoked and expired rows, so callers can classify the failure.
	FindRefreshSession(ctx context.Context, tokenHash string) (entity.RefreshSession, error)
	// RotateRefreshSession atomically revokes the active, unexpired session
	// matching oldTokenHash and inserts next in the same transaction. It returns
	// the consumed session and ErrRefreshSessionNotFound when no usable session
	// matches.
	RotateRefreshSession(
		ctx context.Context,
		oldTokenHash string,
		next entity.RefreshSession,
		now int64,
	) (entity.RefreshSession, error)
	// RevokeRefreshSession revokes the session matching tokenHash with reason
	// and clears any rotation grace metadata. Revoking an unknown or already
	// revoked session is a no-op.
	RevokeRefreshSession(ctx context.Context, tokenHash, reason string, revokedAt int64) error
	// RevokeUserRefreshSessions revokes every active session for userID with
	// reason and clears any rotation grace metadata. It is used by security
	// flows (password change, admin reset, deactivation) that must bypass the
	// rotation grace window.
	RevokeUserRefreshSessions(ctx context.Context, userID int, reason string, revokedAt int64) error
	// RevokeRefreshSessionFamily revokes every active session in familyID with
	// reason and clears any rotation grace metadata. It contains a replay to the
	// affected login chain only.
	RevokeRefreshSessionFamily(ctx context.Context, familyID, reason string, revokedAt int64) error
	// DeleteExpiredRefreshSessions removes sessions whose expiry is at or
	// before now. Active sessions are never removed.
	DeleteExpiredRefreshSessions(ctx context.Context, now int64) error
}
