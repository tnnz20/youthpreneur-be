# Admin Seeder and Member User Listing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an environment-driven admin seeder and exclude admin accounts from `GET /users` while preserving direct admin lookup by public ID.

**Architecture:** Add a standalone `cmd/seeder` CLI that loads existing PostgreSQL configuration, validates required seed credentials, hashes the password, and inserts user/profile atomically. Enforce member-only list behavior in the repository SQL so cursor pagination remains correct; keep `GET /users/{publicID}` unchanged.

**Tech Stack:** Go, `database/sql`, pgx, Viper, bcrypt, existing migration/repository patterns.

**Spec:** User-approved seeder and member-list requirements in conversation.

## Global Constraints

- Seeder credentials come from `SEEDER_ADMIN_EMAIL` and `SEEDER_ADMIN_PASSWORD`.
- Missing or empty credentials return an error; no password logging.
- Admin public IDs use existing `YTP-` plus six digits format.
- User/profile insert is one transaction.
- Existing admin email returns an error and creates no duplicate.
- Profile defaults are seeder variables, not request input.
- `GET /users` excludes `role=admin` in database query.
- `GET /users/{publicID}` remains able to return active admins.
- Cursor pagination remains based on internal `users.id`.

### Task 1: Admin seeder CLI

**Files:**
- Create: `cmd/seeder/main.go`
- Create: `cmd/seeder/main_test.go`
- Modify: `Makefile`, `.env.example`, `README.md`

- [ ] Read existing config and PostgreSQL DSN conventions.
- [ ] Require `SEEDER_ADMIN_EMAIL` and `SEEDER_ADMIN_PASSWORD` from environment.
- [ ] Define profile default variables: `System Administrator`, empty optional profile values, and default gender only if valid under current model.
- [ ] Normalize email and validate password length using existing rules.
- [ ] Generate unique `YTP-` public ID using crypto randomness and bounded collision retries.
- [ ] Hash password with bcrypt.
- [ ] Insert admin user with `role=admin` and profile in one transaction.
- [ ] Map duplicate email to clear error and avoid partial writes.
- [ ] Log success with email and public ID only; never log password/hash.
- [ ] Close DB resources and propagate context/errors.
- [ ] Add tests for missing env, invalid password, duplicate email, transaction failure, and success logging safety.

### Task 2: Exclude admins from user list

**Files:**
- Modify: `internal/repository/persistence/user_postgres.go`
- Modify: `internal/repository/user_repository.go`
- Modify: `internal/usecase/user_usecase.go`
- Modify: `internal/repository/persistence/user_test.go`
- Modify: `internal/usecase/user_test.go`
- Modify: `api/api-contract.md`

- [ ] Add `u.role <> 'admin'` to member-list query.
- [ ] Apply exclusion before cursor and limit so pagination has no gaps.
- [ ] Update repository/usecase/API comments to state active member users.
- [ ] Add tests proving admins are omitted and member cursor pagination remains correct.
- [ ] Preserve `FindUserByPublicID` behavior for active admin lookup.

### Task 3: Documentation and commands

**Files:**
- Modify: `Makefile`, `.env.example`, `README.md`, `AGENTS.md`, `api/api-contract.md`

- [ ] Document seeder command and required environment variables.
- [ ] Document profile defaults and duplicate-admin behavior.
- [ ] Document that `GET /users` lists active members only while direct public-ID lookup may return admins.
- [ ] Ensure no credentials or example secrets are committed.

### Task 4: Verification

- [ ] Run `gofmt -w .` without unrelated formatting churn where possible.
- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run integration seeder/list tests when `TEST_POSTGRES_DSN` is available.
- [ ] Run `git diff --check`.
- [ ] Inspect duplicate code, unused code, transaction safety, and secret handling.
