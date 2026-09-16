# User Service and Database Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add PostgreSQL migrations, a migration CLI, and user/profile service operations with cursor pagination.

**Architecture:** Keep reference-aligned layers: HTTP delivery → usecase → repository → PostgreSQL. `cmd/migrate` owns migration CLI startup. User data uses parameterized `database/sql` queries, Unix epoch seconds, soft deletion, and cursor pagination based on `users.id`.

**Tech Stack:** Go, `database/sql`, PostgreSQL, pgx stdlib driver, golang-migrate/migrate v4, net/http.

**Spec:** Approved user requirements in conversation.

## Global Constraints

- Work on branch `feat/user`.
- Use PostgreSQL and `database/sql`; no ORM.
- Add `github.com/golang-migrate/migrate/v4` and `github.com/jackc/pgx/v5`.
- No authentication or middleware.
- `email` must be unique.
- `created_at`, `updated_at`, and `deleted_at` are Unix epoch seconds in `BIGINT` columns.
- `deleted_at` is NULL until soft delete.
- Public IDs use `YTP-` plus six random digits and must be collision-checked.
- Find-all uses cursor pagination based on `users.id`, never offset pagination.
- Passwords are hashed before persistence and never returned in API responses.

---

### Task 1: Database Migration CLI and Schema

**Files:**
- Modify: `go.mod`
- Create: `cmd/migrate/main.go`
- Create: `migrations/000001_create_users_and_profiles.up.sql`
- Create: `migrations/000001_create_users_and_profiles.down.sql`
- Modify: `internal/config/config.go`
- Modify: `.env.example`
- Modify: `Makefile`

**Interfaces:**
- `cmd/migrate` accepts `up`, `down`, `force VERSION`, and `version`.
- Migration database URL uses configured PostgreSQL values.
- `migrations` is embedded or resolved reliably when command runs from repository root.

- [ ] Add migrate v4 and pgx stdlib dependencies.
- [ ] Add PostgreSQL connection configuration sufficient to build a DSN.
- [ ] Create `user_role` enum with `admin` and `member`.
- [ ] Create `gender_type` enum with `male` and `female`.
- [ ] Create `users` with unique email, public ID, role, active state, Unix timestamps, and nullable soft-delete timestamp.
- [ ] Create `user_profiles` with user foreign key cascade, nullable profile fields, gender enum, and Unix timestamps.
- [ ] Add useful indexes for public ID and profile filters.
- [ ] Down migration drops profiles, users, and enums in dependency order.
- [ ] Implement CLI error handling and migration no-change handling.
- [ ] Add Makefile migration targets without changing existing compose targets.
- [ ] Update `.env.example` with migration-relevant settings if needed.

### Task 2: User Entities and Repository

**Files:**
- Create: `internal/entity/user.go`
- Create: `internal/repository/user.go`
- Create: `internal/repository/user_postgres.go`
- Create: `internal/repository/user_test.go`

**Interfaces:**
- Repository methods accept `context.Context`.
- Repository persists timestamps as Unix seconds.
- Repository methods exclude soft-deleted users unless explicitly required.
- Find-all accepts optional district/gender filters, cursor ID, and limit; returns users plus next cursor.

- [ ] Define user/profile entities without password exposure in response models.
- [ ] Define repository errors for not found and duplicate email/public ID.
- [ ] Implement create user and profile in one transaction.
- [ ] Implement soft delete by public ID.
- [ ] Implement find by public ID with profile.
- [ ] Implement profile update, status update, password change, and password reset persistence.
- [ ] Implement cursor query using `WHERE users.id > $cursor ORDER BY users.id ASC LIMIT $limitPlusOne`.
- [ ] Use parameterized SQL and scan nullable columns safely.
- [ ] Add repository tests using sqlmock only if already acceptable; otherwise keep SQL integration coverage in a PostgreSQL-dependent test path.

### Task 3: User Usecase and Public ID/Password Logic

**Files:**
- Create: `internal/usecase/user.go`
- Create: `internal/usecase/user_test.go`
- Modify: `go.mod`

**Interfaces:**
- Usecase methods cover create, soft delete, find by public ID, profile update, status update, change password, reset password, and cursor find-all.
- Public ID format is `YTP-` plus six decimal digits.
- Find-all response includes `next_cursor`.

- [ ] Add password hashing dependency already suitable for project; use bcrypt or Argon2id without exposing hashes.
- [ ] Generate secure random six-digit suffix with `crypto/rand`.
- [ ] Retry public ID generation on repository duplicate collision.
- [ ] Validate user input at usecase boundary.
- [ ] Set created/updated Unix timestamps in usecase/repository consistently.
- [ ] Make soft delete idempotency behavior explicit.
- [ ] Enforce cursor limit bounds and return next cursor only when another row exists.
- [ ] Add tests for public ID format, duplicate retry, filters, soft delete, status changes, password operations, and cursor behavior.

### Task 4: HTTP Delivery and Bootstrap

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/delivery/http/handler/user.go`
- Modify: `internal/delivery/http/route/route.go`
- Modify: `internal/config/bootstrap.go`
- Modify: `cmd/web/main.go`
- Create: `internal/delivery/http/handler/user_test.go`

**Interfaces:**
- Add REST routes for all eight user operations.
- Request/response JSON never includes password or password hash.
- List endpoint accepts `district`, `gender`, `cursor`, and `limit` query parameters.

- [ ] Define request and response DTOs with validation-friendly fields.
- [ ] Add handlers for create, delete, get, profile update, status update, password change, password reset, and list.
- [ ] Return correct HTTP status codes and JSON errors.
- [ ] Parse cursor and limit safely; reject malformed values.
- [ ] Wire PostgreSQL repository and user usecase in bootstrap while preserving health route.
- [ ] Ensure application startup fails clearly if database dependency cannot initialize.
- [ ] Add handler tests for health compatibility, create, get, and cursor list behavior.

### Task 5: Documentation and Verification

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `.env.example`

- [ ] Document migration commands and user API routes.
- [ ] Document cursor pagination and Unix timestamp behavior.
- [ ] Document no-auth development limitation.
- [ ] Run migrations up/down against PostgreSQL when available.
- [ ] Run `gofmt -w .`.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Inspect status and diff; do not commit unless explicitly requested.
