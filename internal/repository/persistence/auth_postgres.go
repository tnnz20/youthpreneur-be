package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tnnz20/youthpreneur-be/internal/entity"
	"github.com/tnnz20/youthpreneur-be/internal/repository"
)

const refreshSessionColumns = `
	id, user_id, token_hash, expires_at, created_at, revoked_at, replaced_by_hash`

type refreshSessionRepository struct {
	db *sql.DB
}

// NewRefreshSessionRepository creates a PostgreSQL-backed refresh session
// repository.
func NewRefreshSessionRepository(db *sql.DB) repository.RefreshSessionRepository {
	return refreshSessionRepository{db: db}
}

func (r refreshSessionRepository) CreateRefreshSession(ctx context.Context, session entity.RefreshSession) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_sessions (user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert refresh session: %w", err)
	}

	return nil
}

func (r refreshSessionRepository) FindRefreshSession(
	ctx context.Context,
	tokenHash string,
) (entity.RefreshSession, error) {
	query := `SELECT ` + refreshSessionColumns + ` FROM refresh_sessions WHERE token_hash = $1`

	session, err := scanRefreshSession(r.db.QueryRowContext(ctx, query, tokenHash).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.RefreshSession{}, repository.ErrRefreshSessionNotFound
	}
	if err != nil {
		return entity.RefreshSession{}, fmt.Errorf("find refresh session: %w", err)
	}

	return session, nil
}

func (r refreshSessionRepository) RotateRefreshSession(
	ctx context.Context,
	oldTokenHash string,
	next entity.RefreshSession,
	now int64,
) (entity.RefreshSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.RefreshSession{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		id        int
		userID    int
		createdAt int64
	)
	err = tx.QueryRowContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = $2, replaced_by_hash = $3
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
		RETURNING id, user_id, created_at`,
		oldTokenHash,
		now,
		next.TokenHash,
	).Scan(&id, &userID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.RefreshSession{}, repository.ErrRefreshSessionNotFound
	}
	if err != nil {
		return entity.RefreshSession{}, fmt.Errorf("revoke refresh session: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO refresh_sessions (user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`,
		userID,
		next.TokenHash,
		next.ExpiresAt,
		next.CreatedAt,
	); err != nil {
		return entity.RefreshSession{}, fmt.Errorf("insert rotated refresh session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.RefreshSession{}, fmt.Errorf("commit transaction: %w", err)
	}

	revokedAt := now

	return entity.RefreshSession{
		ID:             id,
		UserID:         userID,
		TokenHash:      oldTokenHash,
		CreatedAt:      createdAt,
		RevokedAt:      &revokedAt,
		ReplacedByHash: next.TokenHash,
	}, nil
}

func (r refreshSessionRepository) RevokeRefreshSession(
	ctx context.Context,
	tokenHash string,
	revokedAt int64,
) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
		revokedAt,
	); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}

	return nil
}

func (r refreshSessionRepository) RevokeUserRefreshSessions(
	ctx context.Context,
	userID int,
	revokedAt int64,
) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
		revokedAt,
	); err != nil {
		return fmt.Errorf("revoke user refresh sessions: %w", err)
	}

	return nil
}

func (r refreshSessionRepository) DeleteExpiredRefreshSessions(ctx context.Context, now int64) error {
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM refresh_sessions
		WHERE expires_at <= $1`,
		now,
	); err != nil {
		return fmt.Errorf("delete expired refresh sessions: %w", err)
	}

	return nil
}

// scanRefreshSession maps one refresh_sessions row. scan is sql.Row.Scan or
// sql.Rows.Scan.
func scanRefreshSession(scan func(dest ...any) error) (entity.RefreshSession, error) {
	var (
		session        entity.RefreshSession
		revokedAt      sql.NullInt64
		replacedByHash sql.NullString
	)

	if err := scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
		&revokedAt,
		&replacedByHash,
	); err != nil {
		return entity.RefreshSession{}, err
	}

	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Int64
	}
	if replacedByHash.Valid {
		session.ReplacedByHash = replacedByHash.String
	}

	return session, nil
}
