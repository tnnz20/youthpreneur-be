# PR #2 Authentication Hardening Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve every actionable finding from `docs/pr-review/pr-2-review.md` without changing current cookie `Secure` behavior.

**Architecture:** Keep existing auth layers and repository interfaces. Wire refresh-session revocation into password/account security events, use current DB role for authorization, add replay-family revocation and session cleanup, consolidate shared HTTP error writing, and improve configuration/docs/tests.

**Tech Stack:** Go standard library, `database/sql`, PostgreSQL, Viper, bcrypt, `github.com/golang-jwt/jwt/v5`.

**Spec:** `docs/pr-review/pr-2-review.md`

## Global Constraints

- Missing or empty `APP_AUTH_SECRET` must return startup validation error; do not use default secret in any environment.
- Cookie `HttpOnly=true` always.
- Keep current cookie `Secure` behavior: only exact `APP_ENV=production` enables it.
- Access token lifetime remains 15 minutes; refresh token lifetime remains 7 days.
- Store only SHA-256 refresh-token hashes.
- Propagate request context through handler → usecase → repository.
- Do not trust stale JWT role claims when current user row is already loaded.
- Do not add trusted-proxy parsing or distributed rate-limit storage in this plan; document current direct single-process deployment constraint.
- No production comments unless existing project convention requires them.

### Task 1: Secure auth-secret validation

**Files:** `internal/config/config.go`, `internal/config/config_test.go`, `.env.example`, `README.md`, `AGENTS.md`.

- [ ] Remove usable default auth secret from runtime configuration.
- [ ] Make `Validate()` return an error when `APP_AUTH_SECRET` is missing or empty, including development; preserve minimum-length validation for non-development environments.
- [ ] Add tests for missing, empty, short production, and valid secrets.
- [ ] Update config documentation to state secret is required and never commit credentials.
- [ ] Run `go test ./internal/config/...`.

### Task 2: Revoke sessions after password and account security changes

**Files:** `internal/usecase/user_usecase.go`, `internal/usecase/user_test.go`, `internal/repository/auth_repository.go`, relevant mocks/fakes.

- [ ] Inject `repository.RefreshSessionRepository` into user usecase construction using existing dependency wiring.
- [ ] After successful `ChangePassword`, call `RevokeUserRefreshSessions` with request context and current timestamp.
- [ ] After successful `ResetPassword`, call the same revocation method.
- [ ] After successful `UpdateStatus` changing account to inactive, revoke all sessions.
- [ ] Preserve operation error semantics: if password/status mutation succeeds but revocation fails, return wrapped error and do not expose secrets.
- [ ] Add unit tests proving sessions are revoked for password change, reset, and deactivation.
- [ ] Run focused usecase tests.

### Task 3: Refresh replay family revocation and cleanup

**Files:** `internal/usecase/auth_usecase.go`, `internal/repository/auth_repository.go`, `internal/repository/persistence/auth_postgres.go`, `internal/usecase/auth_test.go`, persistence integration tests.

- [ ] When lookup finds a revoked refresh session, revoke all refresh sessions for its user before returning invalid-token error; do not reveal token state to caller.
- [ ] Keep concurrent rotation atomic and ensure replay tests verify replacement sessions become unusable.
- [ ] Add repository method for deleting expired/revoked sessions, or use one parameterized cleanup statement inside existing login/rotation transaction boundary.
- [ ] Run opportunistic cleanup during login and refresh rotation without deleting active sessions.
- [ ] Add unit and gated integration tests for family revocation and cleanup boundaries.
- [ ] Run focused tests and `go test ./internal/repository/persistence/...`.

### Task 4: Use current DB role for authorization

**Files:** `internal/delivery/http/middleware/auth.go`, middleware tests, route tests.

- [ ] Store current user identity and role loaded by `Authenticate` in typed request context.
- [ ] Make `RequireAdmin` use current DB role, not only JWT role claim.
- [ ] Ensure stale admin token is rejected after DB demotion while active member token remains allowed only for self routes.
- [ ] Add regression tests for stale role claims and inactive/deleted users.
- [ ] Run middleware and route tests.

### Task 5: Timing and token/cookie cleanup

**Files:** `internal/usecase/auth_usecase.go`, `internal/token/token.go`, auth tests, handler tests.

- [ ] Perform bcrypt comparison before inactive-account rejection so unknown, wrong-password, and inactive paths share password-work behavior.
- [ ] Remove redundant JWT signing-method type assertion if `jwt.WithValidMethods` fully enforces HS256.
- [ ] Test inactive login path and invalid/expired cookie max-age behavior.
- [ ] Keep current `APP_ENV`-based `Secure` behavior unchanged.
- [ ] Run auth and token tests.

### Task 6: Remove duplicate/dead API surface and align middleware

**Files:** `internal/delivery/http/handler/response.go`, `internal/delivery/http/middleware/auth.go`, `internal/delivery/http/middleware/ratelimit.go`, tests, bootstrap wiring.

- [ ] Consolidate middleware error responses onto one shared helper without creating an import cycle; use existing response model.
- [ ] Make `RateLimiter.Allow` private if no production caller requires exported access, updating tests through middleware behavior.
- [ ] Ensure disallowed CORS requests are covered by rate limiting where practical through middleware ordering, without changing CORS policy.
- [ ] Add concurrent rate-limit test and run with `go test -race ./internal/delivery/http/middleware/...`.
- [ ] Preserve fixed-window limits and `Retry-After` behavior.

### Task 7: Documentation and operational constraints

**Files:** `api/api-contract.md`, `README.md`, `AGENTS.md`, `CHANGELOG.md`, `docs/pr-review/pr-2-review.md` if status notes are needed.

- [ ] Replace hardcoded rate-limit claims with configured defaults and explain `APP_RATE_LIMIT_*` overrides.
- [ ] Document `APP_ENVIRONMENT` to `APP_ENV` migration if relevant to existing users; use `APP_ENV` consistently.
- [ ] Document exact current cookie rule: `Secure=true` only for `APP_ENV=production`; do not broaden to staging.
- [ ] Document rate limiter limitation: direct single-process deployment only; reverse proxies need trusted client-IP handling and shared storage before production scale-out.
- [ ] Clarify password change is self-service; admin uses password reset for another user.
- [ ] Document refresh-session cleanup and revocation behavior.

### Task 8: Full verification and review

**Files:** none beyond previous tasks.

- [ ] Run `gofmt -w .`.
- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./...` where environment supports it.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Inspect `git diff`, `git status`, and changed-file scope.
- [ ] Confirm every finding in `docs/pr-review/pr-2-review.md` is fixed or explicitly documented as deferred by user decision.
