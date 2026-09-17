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
	// RevokeRefreshSession revokes the session matching tokenHash. Revoking an
	// unknown or already revoked session is a no-op.
	RevokeRefreshSession(ctx context.Context, tokenHash string, revokedAt int64) error
	// RevokeUserRefreshSessions revokes every active session for userID.
	RevokeUserRefreshSessions(ctx context.Context, userID int, revokedAt int64) error
}
