# Code Review: Refresh Grace Window (`c90ba34`)

**Commit:** `c90ba34` — feat(auth): add refresh grace window
**Parent:** `c138aaf` — feat(auth): add current user endpoint
**Scope reviewed:** full diff (16 files, +1058/−74) plus refresh/auth/migration/config/test code in context.
**Date:** 2026-09-20

---

## Conclusion

Solid, well-documented implementation of the rotation grace window. Correctly separates
rotation grace (10s, reason `rotated`) from hard security revocations, scopes replay
containment to `family_id` instead of the whole user, seals the replacement token with
AES-256-GCM keyed from `APP_AUTH_SECRET`, and keeps the rotate transaction atomic.
`go vet` clean, unit tests pass (`go test ./...` with `-count=1` on grace/auth/token suites),
`git diff --check` clean. AGENTS.md and `api/api-contract.md` updated accurately.

No Critical findings. Two Important findings (in-flight key-rotation gap; pre-existing
whole-user revoke race made reachable via `ChangePassword` before token is presented).
Recommend addressing the key-rotation gap before relying on grace across an `APP_AUTH_SECRET`
change, otherwise ship-ready.

---

## Strengths

- Grace path is read-only: `serveGraceWindow` never writes, so repeated duplicates cannot
  extend `grace_until` (`internal/usecase/auth_usecase.go:296`).
- Rotation stores the encrypted replacement before any write; decrypt failure at serve time
  fails closed with no token issuance (`auth_usecase.go:312`).
- Replay containment narrowed from whole-user to family — smaller blast radius, matches new
  contract language in `api/api-contract.md`.
- Security revocations (`logout`, `password_change`, `admin_reset`, `deactivated`) never
  open a window; only `ReasonRotated` does (`internal/entity/auth.go`).
- Migration `000006` is additive: `DEFAULT gen_random_uuid()` backfills `family_id` for
  existing rows; down migration drops columns/indexes in dependency-safe order.
- Integration test proves rotate rollback atomicity (duplicate replacement hash) and that
  family revoke spares a second family of the same user (`auth_test.go`).
- Grace tests use injected clock — deterministic, no sleeps; boundary at exactly 10s
  (inclusive) and 11s (replay) covered.

---

## Findings

### Important

**I-1. Grace-window ciphertext is unreadable after `APP_AUTH_SECRET` rotation.**
`refreshCipher` derives the AES-256-GCM key as `sha256(s.secret)` at call time
(`internal/token/token.go:155`). If the secret changes between a rotation and a duplicate
retry (deploy rollover), `DecryptRefresh` fails and the retry gets `500` instead of the
grace response. Window is only 10s, so exposure is narrow, but it is a fail-closed gap, not
a security hole. Suggestion: document the constraint (secret must be stable across a
deployment) or return `ErrInvalidRefreshToken` instead of `500` when grace decryption
fails. One line: wrap the decrypt error into `ErrInvalidRefreshToken` in `serveGraceWindow`
(an unwrappable secret means the retry cannot be served anyway).

**I-2. In-flight rotation race: grace and replay checks are not serialized with rotate.**
`Refresh` reads the session, sees it active, then `RotateRefreshSession` re-checks
atomically — a concurrent duplicate that loses the race gets `ErrRefreshSessionNotFound`
→ `401` instead of a grace response. Correct but means "duplicate within grace always
succeeds" has a narrow concurrent exception. More notable: `RevokeUserRefreshSessions`
(password change, reset, deactivation) and the grace read in `Refresh` do not take a row
lock, so a deactivation committing between the grace check and the response still lets the
grace response through. Mitigated: `serveGraceWindow` re-checks `user.IsActive` after the
revoke, and the access token TTL is 15 min regardless, so real exposure is a ~10s window
where an already-revoked family can mint one more access token. Document as accepted risk
or lock the session row (`SELECT ... FOR UPDATE`) in the grace path. Low urgency.

### Minor

**M-1. `RevokeUserRefreshSessions` does not clear grace metadata.**
A row revoked for `password_change` keeps a stale `grace_until`/`replacement_token_enc` from
an earlier rotation. The usecase check `RevocationReason == ReasonRotated` correctly gates
grace on the *current* reason, so this is safe today, but the leftover ciphertext is dead
data (and stored plaintext-equivalent material that outlives its purpose). Suggestion: also
`SET grace_until = NULL, replacement_token_enc = NULL` in the security-revoke statements.

**M-2. `ReplacedByHash` is now redundant.**
`family_id` + `revocation_reason` supersede it for replay logic; it is written in rotate and
scanned everywhere but read nowhere (`grep` over `c90ba34` shows no reader outside entity
mapping). Candidate for removal in a future migration; harmless today.

**M-3. `newFamilyID` hand-rolls UUIDv4.**
Stdlib `crypto/rand` + manual bit-twiddling in `auth_usecase.go:370`. Go 1.27 has no
stdlib UUID, so this is fine; note `db` also uses `gen_random_uuid()` (pgcrypto built-in in
PG13+), so the two formats are consistent. No action needed — flagging only because
`github.com/google/uuid` would be an unnecessary dependency per repo convention.

**M-4. `go vet` clean; `gofmt` unformatted files predate this commit.**
`gofmt -l` flags 24 files, none touched by `c90ba34` (verified per-file). Line-ending/EOF
noise on Windows; not a blocker for this review.

**M-5. Handler-level grace test missing.**
`auth_handler_test.go` covers rotate/cookie flows but no test asserts a duplicate refresh
inside grace returns `204` with the *same* refresh cookie value. Usecase-level coverage is
strong; the cookie round-trip on the grace path is untested. One table-driven case would
close it.

**M-6. API contract says grace returns "the same replacement refresh token with a fresh
access token", and `RefreshExpiresAt` is served from `session.ExpiresAt`.**
Correct — but note the duplicate client gets a `Set-Cookie` with an already-aging cookie
expiring at the original replacement's expiry. Contract language is accurate; no change
needed, just confirming the nuance was intentional.

---

## Test Gaps

- No handler test for the grace duplicate path (`204` + same refresh cookie).
- No test that a `password_change` revoke after rotation kills the grace window *for the
  new token's duplicates* (reason is overwritten to `password_change`, so covered
  transitively by reason-gating, but an explicit test would pin intent).
- Integration tests (`TEST_POSTGRES_DSN`) were not run here — no DSN available in this
  environment; unit suites cover the logic, DB-level atomicity covered by the rollback
  test when DSN is provided.
- No test for concurrent duplicate rotations (I-2 behavior).

---

## Security Notes

- AES-256-GCM with random per-write nonce; tamper/wrong-key/short-ciphertext all tested
  (`internal/token/grace_test.go`). Key derivation via `sha256(secret)` is acceptable given
  secret ≥32 bytes enforced outside dev; a KDF (HKDF) would be more principled but is not
  warranted for a 10s-lived ciphertext.
- Raw replacement token never logged; ciphertext stored as `BYTEA`.
- Grace does not apply to logout/password-change/admin-reset/deactivation — verified by
  reason gating and `TestSecurityRevocationsBypassGrace`.
- Replay after grace revokes family only — verified by `TestPostGraceReplayRevokesOnlyFamily`.

---

## Migration Compatibility

- `000006` up: additive columns with `DEFAULT gen_random_uuid()` — no table rewrite risk
  flagged for PG16 (compose image `postgres:16-alpine`); `gen_random_uuid()` is built in
  from PG13. Partial index `idx_refresh_sessions_family_active` matches replay-revoke
  predicate (`revoked_at IS NULL`).
- `000006` down: drops indexes then columns; safe.
- Existing in-flight refresh sessions at deploy time: backfilled `family_id` is random per
  row, so an old row and its replacement row get *different* family IDs — replay
  containment for pre-migration chains is per-session, not per-family. Self-heals as
  sessions rotate post-deploy. Acceptable; note for ops.

## AGENTS.md / Contract Accuracy

- AGENTS.md refresh paragraph accurately describes 10s window, encrypted replacement, key
  derivation from `APP_AUTH_SECRET`, non-rotation reasons, family-scoped replay, epoch-second
  precision, and migration `000006`.
- `api/api-contract.md` refresh section updated: grace semantics, family revoke scope,
  encryption note, and status/cookie behavior all match `route.go`/`auth_handler.go`
  behavior (204 + Set-Cookie on grace success, 401 otherwise).

---

## Suggestions (ordered)

1. In `serveGraceWindow`, map decrypt failure to `ErrInvalidRefreshToken` (I-1 partial fix,
   one line).
2. Clear `grace_until`/`replacement_token_enc` in security-revoke SQL (M-1).
3. Add handler-level grace duplicate test (M-5).
4. Document `APP_AUTH_SECRET` stability requirement across deploys (I-1 remainder).
5. Consider dropping `ReplacedByHash` in a future migration (M-2).

## Readiness

**Ship-ready with follow-ups.** No Critical issues. I-1's deploy-rollover edge and I-2's
documented race are narrow and fail closed; items 1–3 above are small, low-risk follow-ups.
Production code untouched during review.
