# Seeder Review Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve PR #5 review documentation drift and minor seeder cleanup without changing approved behavior.

**Architecture:** Keep seeder transaction and member-list SQL unchanged. Update API/agent documentation to match behavior, consolidate exported user validation helpers with their underlying implementations, and make profile defaults immutable/local. Run integration verification when PostgreSQL is available.

**Tech Stack:** Go, PostgreSQL, bcrypt, existing Viper and repository patterns.

**Spec:** `docs/pr-review/pr-5-review.md`

## Global Constraints

- `GET /users` excludes admin accounts before cursor and limit.
- `GET /users/{publicID}` may return active admins.
- Seeder uses `SEEDER_ADMIN_EMAIL` and `SEEDER_ADMIN_PASSWORD`.
- Seeder never logs password or password hash.
- User/profile creation remains transactional.
- No unrelated auth or API behavior changes.

### Task 1: Align API and agent documentation

**Files:**
- Modify: `api/api-contract.md`
- Modify: `AGENTS.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md` if release notes need alignment

- [ ] Change user list wording to “active member users” and state admin accounts are excluded.
- [ ] State direct public-ID lookup may return active admin accounts.
- [ ] Document seeder environment variables and process-environment requirement.
- [ ] Add password rotation guidance after initial admin provisioning.
- [ ] Add tests only if docs are generated or contract assertions exist.

### Task 2: Remove duplicate user helper wrappers

**Files:**
- Modify: `internal/usecase/user_usecase.go`
- Modify: `cmd/seeder/main.go`
- Modify: related unit tests

- [ ] Consolidate exported `NormalizeEmail`, `ValidateEmail`, `ValidatePassword`, and `GeneratePublicID` with their underlying implementations without changing callers or behavior.
- [ ] Preserve email normalization, validation errors, password byte limits, and public-ID retry behavior.
- [ ] Ensure seeder uses the consolidated helpers.
- [ ] Run focused usecase and seeder tests.

### Task 3: Make seeder profile defaults non-mutable

**Files:**
- Modify: `cmd/seeder/main.go`
- Modify: `cmd/seeder/main_test.go` if needed

- [ ] Replace package-level mutable `adminProfile` with a local value or constructor function.
- [ ] Preserve `System Administrator` full name and empty optional fields.
- [ ] Confirm no password/hash appears in output or errors.

### Task 4: Integration verification

**Files:** none unless tests need changes.

- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `gofmt` on changed Go files.
- [ ] Run `git diff --check`.
- [ ] Run user-list and seeder integration tests with `TEST_POSTGRES_DSN` when available.
- [ ] Inspect diff for duplicate/unused code and unintended behavior changes.
