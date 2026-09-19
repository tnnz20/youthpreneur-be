# Authenticated GET /auth/me Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add authenticated `GET /auth/me` returning the minimal safe current-user shape from the database-loaded identity.

**Architecture:** Reuse the existing `Authenticate` middleware, which already loads the active database user into the request context. The handler reads that identity through the injected `IdentityFunc`, so no new repository or usecase method is added and the JWT role claim is never trusted. A dedicated response model keeps the shape minimal and independent from `UserResponse`.

**Tech Stack:** Go, standard library `net/http`, existing handler/middleware/route patterns.

**Spec:** Approved requirements in conversation.

## Global Constraints

- Response shape is exactly `public_id`, `email`, `role`, and `profile.full_name`.
- `profile` is `null` when the user has no profile.
- Never include `is_active`, timestamps, password, tokens, or other profile fields.
- Role comes from the database-loaded identity, never the JWT role claim.
- Missing identity returns `401`.
- No new repository or usecase methods.
- No comments unless the existing style already uses one.

### Task 1: Minimal response model

**Files:**
- Modify: `internal/model/auth.go`

- [x] Add `MeProfileResponse` with only `full_name`.
- [x] Add `MeResponse` with `public_id`, `email`, `role`, and non-omitted `profile`.
- [x] Keep `MeResponse` separate from `UserResponse` so no extra fields leak.

### Task 2: Handler and route

**Files:**
- Modify: `internal/delivery/http/handler/auth_handler.go`
- Modify: `internal/delivery/http/route/route.go`
- Modify: `internal/config/bootstrap.go`

- [x] Inject `IdentityFunc` into `AuthHandler` so the handler package does not import `middleware`.
- [x] Add `Me` handler reading identity from context; missing identity returns `401`.
- [x] Add `toMeResponse` mapping, serializing a nil profile as JSON `null`.
- [x] Register `GET /auth/me` behind `Authenticate`.
- [x] Update `NewAuthHandler` call sites for the new parameter.

### Task 3: Tests

**Files:**
- Modify: `internal/delivery/http/handler/protected_route_test.go`
- Modify: `internal/delivery/http/route/route_test.go`

- [x] Test the minimal shape through the real middleware, asserting no leaked fields.
- [x] Test that the database role wins over a stale JWT role claim.
- [x] Test missing profile serializes as `null`.
- [x] Test missing identity returns `401`.
- [x] Test `GET /auth/me` is registered.

### Task 4: Documentation

**Files:**
- Modify: `api/api-contract.md`

- [x] Add `GET /auth/me` to the route permission table.
- [x] Document the endpoint, response, authentication, and errors.
- [x] Note the minimal shape and the `null` profile case.

### Task 5: Verification

- [x] Run `gofmt -w` on changed Go files.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `git diff --check`.
- [x] Do not commit.
