# Refresh Grace-Window Review Follow-ups

Date: 2026-09-20
Commit reviewed: `c90ba34` (feat(auth): add refresh grace window)
Review: `docs/superpowers/reviews/2026-09-20-refresh-grace-window-review.md`

## Tasks

- [x] 1. In `serveGraceWindow`, map decrypt failure to `ErrInvalidRefreshToken` so
  `POST /auth/refresh` returns `401`, not `500`; do not leak crypto errors.
- [x] 2. Clear `grace_until` and `replacement_token_enc` in the security-revoke SQL
  (`RevokeRefreshSession`, `RevokeUserRefreshSessions`, `RevokeRefreshSessionFamily`)
  while preserving existing `revocation_reason` semantics.
- [x] 3. Add handler-level test proving a grace duplicate returns `204` and sets the
  same replacement refresh cookie value with preserved cookie attributes.
- [x] 4. Add explicit test proving password-change/security revocation after rotation
  disables grace for the replacement/old-token duplicate path.
- [x] 5. Update `AGENTS.md` and relevant API contract/security docs to state
  `APP_AUTH_SECRET` must remain stable across deployments for the short grace window,
  or document the behavior on rotation. No secret values documented.
- [x] 6. Review I-2 race: do not claim full concurrent duplicate success. Document the
  accepted narrow race in this checklist and docs unless row locking is practical
  without unsafe broad refactor.
- [x] 7. Review stale/redundant `ReplacedByHash`: do not remove schema in this
  follow-up; document as intentionally retained or deferred.
- [x] 8. Run `gofmt` on changed Go files.
- [x] 9. Run `go test ./...`, `go vet ./...`, `git diff --check`; fix change-caused
  failures.

## Findings Addressed / Deferred

- I-1: addressed — decrypt failure now maps to `ErrInvalidRefreshToken` (401); docs
  note `APP_AUTH_SECRET` must stay stable for the grace window.
- I-2: deferred (documented) — see task 6 notes below.
- M-1: addressed — security revokes clear grace metadata (task 2).
- M-2: deferred (documented) — `ReplacedByHash` intentionally retained (task 7).
- M-5: addressed — handler grace duplicate test (task 3).

## Notes

- I-2: `Refresh` reads the session, then `RotateRefreshSession` re-checks
  `revoked_at IS NULL` atomically; a concurrent duplicate that loses the race gets
  `ErrRefreshSessionNotFound` → `401` (not grace). The grace read and the security
  revoke do not share a row lock. Accepted narrow race: a deactivation committing
  between the grace check and response can let one already-revoked family mint one
  more access token; `serveGraceWindow` re-checks `user.IsActive` and access TTL is
  15 minutes, so exposure is bounded to ~10s. Not implemented: row locking in the
  read-only grace path would require restructuring the repository transaction to
  hold `SELECT ... FOR UPDATE` across a read that currently uses `*sql.DB`; the
  narrow race is accepted and documented instead.
- M-2: `replaced_by_hash` is written by rotate and scanned but read by no production
  logic after `family_id`/`revocation_reason` were introduced. Removing it needs a
  migration and is out of scope for this follow-up; retained deliberately.
- Task 2 nuance: a rotated row is already `revoked_at IS NOT NULL`, so a security
  revoke that only matched active rows could not clear its grace metadata. The three
  security-revoke statements now also match already-revoked rows that still carry
  `grace_until`, clear the grace metadata, and preserve their existing
  `revocation_reason` (e.g. an already-`rotated` row keeps `rotated` while losing its
  grace window). This is what makes task 4 hold.

## Verification

- `gofmt -w` on all changed Go files: `gofmt -l` reports nothing.
- `go test ./...`: all packages `ok`, no failures.
- `go vet ./...`: clean.
- `git diff --check`: exit 0 (only CRLF informational warnings on Windows).
- Persistence integration tests remain gated behind `TEST_POSTGRES_DSN` and were not
  run here (no DSN in this environment); the new grace-clearing SQL is mirrored by the
  usecase fake and covered by task 4's unit test.
