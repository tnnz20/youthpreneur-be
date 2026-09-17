# Authentication and Middleware Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add secure cookie JWT authentication, PostgreSQL refresh sessions, authorization, CORS, and rate limiting.

**Architecture:** Add auth models/usecase/repository/handler layers matching existing flow. Middleware validates access JWT and injects claims into request context; route groups apply authentication, role, and ownership checks. Refresh tokens are opaque random values stored only as hashes and rotated transactionally.

**Tech Stack:** Go standard library, PostgreSQL via `database/sql`, Viper, bcrypt, `github.com/golang-jwt/jwt/v5`.

**Spec:** `docs/superpowers/specs/2026-09-17-auth-design.md`

## Global Constraints

- Access token lifetime is 15 minutes; refresh token lifetime is 7 days.
- `HttpOnly` is always true; cookie `Secure` is true only when `APP_ENV=production`.
- Store only SHA-256 refresh-token hashes; revoke consumed/logout tokens.
- Public routes remain health, registration, login, and refresh.
- Passwords and token values never appear in logs or responses.
- Propagate `r.Context()` through handler → usecase → repository and use `QueryContext`/`ExecContext`.
- Run `go test ./...`, `go vet ./...`, and `git diff --check` before completion.

### Task 1: Dependencies and configuration

**Files:** `go.mod`, `go.sum`, `internal/config/config.go`, `.env.example`, config tests.

- [ ] Add `github.com/golang-jwt/jwt/v5`.
- [ ] Add auth secret, `APP_ENV`, CORS origins, and rate-limit settings using existing Viper patterns.
- [ ] Make production cookie security derive from `APP_ENV`.
- [ ] Add failing config tests, then implementation and focused test run.

### Task 2: Refresh-token migration and repository

**Files:** new migration files, `internal/repository/user_repository.go`, new auth repository files, integration tests.

- [ ] Add refresh session table with user FK, token hash uniqueness, expiry, created/revoked timestamps.
- [ ] Define minimal repository methods for create, consume rotation, revoke, and user lookup.
- [ ] Implement parameterized SQL and explicit transactions using context-aware calls.
- [ ] Add integration tests gated by `TEST_POSTGRES_DSN`.

### Task 3: Token service and auth usecase

**Files:** new auth package/usecase/model files, unit tests.

- [ ] Add JWT access claims containing user ID, public ID, role, issuer, issued-at, and expiry.
- [ ] Add opaque refresh generation, SHA-256 hashing, expiry, rotation, and revocation.
- [ ] Implement login with normalized email and bcrypt verification.
- [ ] Implement refresh validation and logout revocation.
- [ ] Add unit tests before implementation for valid/invalid/expired tokens and auth failures.

### Task 4: Auth handlers and routes

**Files:** new auth handler files, `internal/delivery/http/route/route.go`, handler tests, route tests.

- [ ] Add `POST /auth/login`, `POST /auth/refresh`, and `POST /auth/logout`.
- [ ] Set/clear `access_token` and `refresh_token` cookies with `HttpOnly`, environment-derived `Secure`, `SameSite=Lax`, explicit paths, and lifetimes.
- [ ] Use request context and existing JSON/error conventions.
- [ ] Test status codes, cookie attributes, malformed requests, and logout clearing.

### Task 5: Authentication, role, and ownership middleware

**Files:** new middleware files, context tests, route tests.

- [ ] Validate access cookie JWT and reject missing, invalid, expired, inactive, or deleted users.
- [ ] Store typed auth claims using an unexported context key.
- [ ] Add role middleware and ownership middleware allowing admins through.
- [ ] Apply approved route policy without changing public registration.
- [ ] Test middleware decisions and protected route responses.

### Task 6: CORS and rate limiting middleware

**Files:** new middleware files, tests, bootstrap/router wiring.

- [ ] Implement credentialed CORS with local defaults and configured production origins.
- [ ] Handle OPTIONS preflight and reject disallowed origins.
- [ ] Implement per-client-IP fixed-window limits: login 5/minute, refresh 10/minute, general API 60/minute.
- [ ] Return `429` and `Retry-After`; test window reset and endpoint limits.

### Task 7: Documentation and verification

**Files:** `api/api-contract.md`, `AGENTS.md`, `README.md` if configuration sections need alignment.

- [ ] Document auth routes, cookies, `APP_ENV`, refresh rotation/revocation, route permissions, CORS, and rate limits.
- [ ] Remove stale statements saying all routes are public or auth is unimplemented.
- [ ] Run `gofmt -w .`, `go test ./...`, `go vet ./...`, and `git diff --check`.
- [ ] Review `git diff` and working tree; fix all failures before completion.
