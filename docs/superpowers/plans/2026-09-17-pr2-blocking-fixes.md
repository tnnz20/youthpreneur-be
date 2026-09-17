# PR #2 Blocking Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve refresh-token replay detection after cleanup and add regression coverage for inactive-login password verification.

**Architecture:** Keep revoked refresh-session rows until their expiry so replay detection can identify rotated tokens. Keep opportunistic cleanup expiry-only. Add focused usecase tests proving cleanup does not erase replay evidence and inactive login still performs bcrypt verification before rejection.

**Tech Stack:** Go, PostgreSQL via `database/sql`, existing auth usecase and test fakes.

**Spec:** `docs/pr-review/pr-2-review.md`

## Global Constraints

- Delete refresh sessions only when `expires_at <= now`.
- Replayed revoked tokens must revoke the user's refresh-session family.
- Do not change cookie `Secure` behavior.
- Passwords and tokens must not enter logs or responses.
- Run `gofmt`, `go test ./...`, `go vet ./...`, and `git diff --check`.

### Task 1: Preserve replay evidence during cleanup

**Files:**
- Modify: `internal/repository/persistence/auth_postgres.go:155-164`
- Test: `internal/repository/persistence/auth_test.go`

**Interfaces:**
- Preserve `DeleteExpiredRefreshSessions(ctx context.Context, now int64) error`.
- SQL must remove expired rows while retaining unexpired revoked rows.

- [ ] **Step 1: Add failing integration assertion**

Extend the gated persistence test to create one revoked unexpired session and one expired session, call `DeleteExpiredRefreshSessions`, then assert the revoked unexpired row remains and expired row is deleted.

- [ ] **Step 2: Run focused test**

Run: `go test ./internal/repository/persistence -run TestUserRepositoryIntegration -count=1`

Expected: integration test runs only when `TEST_POSTGRES_DSN` is configured; with migrated PostgreSQL, the new assertion fails against cleanup SQL containing `OR revoked_at IS NOT NULL`.

- [ ] **Step 3: Apply minimal SQL fix**

Change cleanup predicate to:

```sql
DELETE FROM refresh_sessions
WHERE expires_at <= $1
```

- [ ] **Step 4: Run focused and full tests**

Run: `go test ./internal/repository/persistence/... -count=1` and `go test ./...`

Expected: PASS; integration remains environment-gated when no DSN exists.

### Task 2: Test inactive-login bcrypt ordering

**Files:**
- Modify: `internal/usecase/auth_test.go`
- Modify: `internal/usecase/auth_usecase.go` only if test exposes ordering regression

**Interfaces:**
- Preserve `AuthUseCase.Login(ctx context.Context, input LoginInput) (LoginResult, error)`.
- Inactive accounts must perform password comparison before returning invalid credentials.

- [ ] **Step 1: Add a deterministic test seam**

Use existing repository/token fakes and add a test-specific password hash or bcrypt spy seam only if current code cannot observe ordering. Prefer asserting an inactive user with the correct password returns `ErrInvalidCredentials` after bcrypt verification path executes, without timing assertions.

- [ ] **Step 2: Run test to verify expected behavior**

Run: `go test ./internal/usecase -run Test.*Inactive.*Login -count=1`

Expected: PASS against current implementation; test locks the corrected behavior and fails if the inactive check moves before password verification.

- [ ] **Step 3: Keep implementation minimal**

Do not change production logic if current order already compares bcrypt before checking `IsActive`. Modify only if test demonstrates a regression.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/usecase -count=1`

Expected: PASS.

### Task 3: Final verification

**Files:** none.

- [ ] Run `gofmt -w internal/repository/persistence/auth_postgres.go internal/repository/persistence/auth_test.go internal/usecase/auth_test.go`.
- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./internal/usecase ./internal/delivery/http/middleware`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Inspect `git diff` and confirm only blocking fix plus test changes exist.
