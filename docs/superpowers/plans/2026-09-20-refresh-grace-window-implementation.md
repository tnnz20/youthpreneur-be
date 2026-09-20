# Refresh Grace-Window Implementation Checklist

Date: 2026-09-20
Branch: feat/auth-2

Assumptions confirmed by requester:

- PostgreSQL 13+, `gen_random_uuid()` available.
- `family_id` is a `UUID` column on `refresh_sessions` (no separate family table).
- Existing rows get unique family IDs via migration default.
- New login creates a new family ID.

## Tasks

- [x] 1. Investigate and preserve existing cookie JWT refresh behavior.
- [x] 2. Add secure persistence for family + grace metadata (migration, entity, repository).
- [x] 3. Implement 10-second grace window for normal rotation only.
- [x] 4. Post-grace old-token use revokes only its token family.
- [x] 5. Security revocations (logout, password change, admin reset, deactivation) bypass grace.
- [x] 6. Preserve expiry, active/deleted checks, 401 semantics, cookie attributes.
- [x] 7. No new external dependencies; reuse `APP_AUTH_SECRET` for encryption key.
- [x] 8. Focused unit/repository tests for all required cases.
- [x] 9. Update `api/api-contract.md` with grace/replay/revocation/401 behavior.
- [x] 10. Update this checklist with results and skipped items.
- [x] 11. Run gofmt on changed Go files.
- [x] 12. Run `go test ./...`, `go vet ./...`, `git diff --check`; fix failures.

## Design

- Migration `000006_add_refresh_session_family_and_grace`: adds
  `family_id UUID NOT NULL DEFAULT gen_random_uuid()`, `revocation_reason`
  `VARCHAR(32) NULL`, `grace_until BIGINT NULL`, `replacement_token_enc BYTEA NULL`,
  plus an index on `family_id` and a partial index on active rows per family.
- Refresh token encryption: AES-256-GCM, key = SHA-256(`APP_AUTH_SECRET`), random
  12-byte nonce prefixed to ciphertext. No new dependency (`crypto/*` stdlib).
- Grace window only applies when `revocation_reason = 'rotated'` and
  `grace_until >= now`. Duplicate old-token request inside grace returns the same
  decrypted replacement refresh token plus a freshly issued access token. Grace
  deadline is never extended.
- Old-token use after grace, or any other revocation reason, is replay: revokes
  only that `family_id`, not all user sessions.

## Verification Commands

- `go test ./...`
- `go vet ./...`
- `git diff --check`

## Results

Changed files:

- `db/migrations/000006_add_refresh_session_family_and_grace.up.sql` (new)
- `db/migrations/000006_add_refresh_session_family_and_grace.down.sql` (new)
- `internal/entity/auth.go`
- `internal/repository/auth_repository.go`
- `internal/repository/persistence/auth_postgres.go`
- `internal/repository/persistence/auth_test.go`
- `internal/token/token.go`
- `internal/token/grace_test.go` (new)
- `internal/usecase/auth_usecase.go`
- `internal/usecase/auth_test.go`
- `internal/usecase/auth_grace_test.go` (new)
- `internal/usecase/user_usecase.go`
- `internal/usecase/user_test.go`
- `api/api-contract.md`

Test results (run 2026-09-20):

- `gofmt -w` on all changed Go files: no output (clean).
- `go test ./...`: all packages `ok`, no failures.
- `go vet ./...`: clean.
- `git diff --check`: exit 0 (only CRLF informational warnings from Git).

Skipped items / limitations:

- Persistence integration tests (new family/grace/rollback assertions in
  `auth_test.go`) are gated behind `TEST_POSTGRES_DSN` and were skipped in this
  run because no test database was configured. Run with
  `TEST_POSTGRES_DSN='...' go test ./internal/repository/persistence` against a
  migrated database to exercise them.
- `grace_until` is stored in epoch seconds, matching the rest of the schema, so
  the grace window has second granularity.
- The grace window is served from the database on each duplicate request; it
  relies on `RotateRefreshSession` already being transactional for the initial
  rotation. Concurrent duplicate requests that both observe an unrotated session
  still serialize through the atomic `UPDATE ... WHERE revoked_at IS NULL`, so only
  one rotation wins and the loser fails with `401` (not grace) — grace covers the
  sequential retry, not the simultaneous race.
