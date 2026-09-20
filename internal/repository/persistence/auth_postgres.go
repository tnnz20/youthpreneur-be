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
	id, user_id, family_id, token_hash, expires_at, created_at, revoked_at,
	revocation_reason, grace_until, replacement_token_enc, replaced_by_hash`

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
		INSERT INTO refresh_sessions (user_id, family_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		session.UserID,
		session.FamilyID,
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
		familyID  string
		createdAt int64
	)
	err = tx.QueryRowContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = $2,
		    revocation_reason = $3,
		    grace_until = $4,
		    replacement_token_enc = $5,
		    replaced_by_hash = $6
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
		RETURNING id, user_id, family_id, created_at`,
		oldTokenHash,
		now,
		entity.ReasonRotated,
		next.GraceUntil,
		next.ReplacementTokenEnc,
		next.TokenHash,
	).Scan(&id, &userID, &familyID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.RefreshSession{}, repository.ErrRefreshSessionNotFound
	}
	if err != nil {
		return entity.RefreshSession{}, fmt.Errorf("revoke refresh session: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO refresh_sessions (user_id, family_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		userID,
		familyID,
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
		ID:                  id,
		UserID:              userID,
		FamilyID:            familyID,
		TokenHash:           oldTokenHash,
		CreatedAt:           createdAt,
		RevokedAt:           &revokedAt,
		RevocationReason:    entity.ReasonRotated,
		GraceUntil:          next.GraceUntil,
		ReplacementTokenEnc: next.ReplacementTokenEnc,
		ReplacedByHash:      next.TokenHash,
	}, nil
}

func (r refreshSessionRepository) RevokeRefreshSession(
	ctx context.Context,
	tokenHash, reason string,
	revokedAt int64,
) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = COALESCE(revoked_at, $2),
		    revocation_reason = CASE WHEN revoked_at IS NULL THEN $3 ELSE revocation_reason END,
		    grace_until = NULL, replacement_token_enc = NULL
		WHERE token_hash = $1 AND (revoked_at IS NULL OR grace_until IS NOT NULL)`,
		tokenHash,
		revokedAt,
		reason,
	); err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}

	return nil
}

func (r refreshSessionRepository) RevokeUserRefreshSessions(
	ctx context.Context,
	userID int,
	reason string,
	revokedAt int64,
) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = COALESCE(revoked_at, $2),
		    revocation_reason = CASE WHEN revoked_at IS NULL THEN $3 ELSE revocation_reason END,
		    grace_until = NULL, replacement_token_enc = NULL
		WHERE user_id = $1 AND (revoked_at IS NULL OR grace_until IS NOT NULL)`,
		userID,
		revokedAt,
		reason,
	); err != nil {
		return fmt.Errorf("revoke user refresh sessions: %w", err)
	}

	return nil
}

func (r refreshSessionRepository) RevokeRefreshSessionFamily(
	ctx context.Context,
	familyID, reason string,
	revokedAt int64,
) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = COALESCE(revoked_at, $2),
		    revocation_reason = CASE WHEN revoked_at IS NULL THEN $3 ELSE revocation_reason END,
		    grace_until = NULL, replacement_token_enc = NULL
		WHERE family_id = $1 AND (revoked_at IS NULL OR grace_until IS NOT NULL)`,
		familyID,
		revokedAt,
		reason,
	); err != nil {
		return fmt.Errorf("revoke refresh session family: %w", err)
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
		session             entity.RefreshSession
		revokedAt           sql.NullInt64
		revocationReason    sql.NullString
		graceUntil          sql.NullInt64
		replacementTokenEnc []byte
		replacedByHash      sql.NullString
	)

	if err := scan(
		&session.ID,
		&session.UserID,
		&session.FamilyID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
		&revokedAt,
		&revocationReason,
		&graceUntil,
		&replacementTokenEnc,
		&replacedByHash,
	); err != nil {
		return entity.RefreshSession{}, err
	}

	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Int64
	}
	if revocationReason.Valid {
		session.RevocationReason = revocationReason.String
	}
	if graceUntil.Valid {
		session.GraceUntil = &graceUntil.Int64
	}
	session.ReplacementTokenEnc = replacementTokenEnc
	if replacedByHash.Valid {
		session.ReplacedByHash = replacedByHash.String
	}

	return session, nil
}
