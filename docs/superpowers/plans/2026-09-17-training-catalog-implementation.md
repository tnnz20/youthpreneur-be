# Training Catalog and Enrollment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add public training catalog browsing, admin catalog management, authenticated enrollment lifecycle, history, capacity safety, and cursor pagination.

**Architecture:** Keep catalog and enrollment as separate services/repositories under existing route → handler → usecase → PostgreSQL layers. Catalog reads are public; catalog writes are admin-only. Enrollment mutations own a transaction that locks the catalog row, checks active capacity, enforces active uniqueness, and preserves cancellation history through `deleted_at`.

**Tech Stack:** Go, PostgreSQL, `database/sql`, existing Viper/auth middleware, standard library HTTP, existing migration tooling.

**Spec:** User-approved training catalog and enrollment requirements in conversation.

## Global Constraints

- Catalog reads are public: no authentication middleware.
- Catalog create, update, status update, and delete are admin-only.
- Enrollment routes require authentication.
- Members enroll/cancel/view only their own enrollment data.
- Admins can view all enrollment history and catalog enrollment history.
- Use `training_catalog_id`; catalog and enrollment have generated public IDs.
- Enrollment cancellation sets `deleted_at`; history remains queryable; re-enrollment is allowed.
- User IDs come from current authenticated identity, never request body/query.
- Use `process_status_enum` for `training_status`.
- Cursor uses internal IDs, default limit 20, maximum 100, query `limit + 1`.
- Capacity checks and enrollment inserts occur in one transaction with catalog row lock.
- All mutation SQL is parameterized and context-aware.

### Task 1: Migration and indexes

**Files:**
- Create: `db/migrations/000005_create_training_catalog_and_enrollments.up.sql`
- Create: `db/migrations/000005_create_training_catalog_and_enrollments.down.sql`

- [ ] Add `training_catalog` with requested fields, generated `public_id`, timestamps, and soft delete.
- [ ] Reuse existing `process_status_enum`; do not recreate it.
- [ ] Add `training_enrollments` with generated `public_id`, `training_catalog_id`, user FK, register date, timestamps, and `deleted_at`.
- [ ] Add positive `training_slots` check.
- [ ] Add partial unique active enrollment index on `(training_catalog_id, user_id) WHERE deleted_at IS NULL`.
- [ ] Add indexes for catalog deleted/date/category/status and cursor; enrollment catalog/user cursor and deleted state.
- [ ] Make down migration dependency-safe and preserve existing shared enum.

### Task 2: Catalog entity, model, repository, and service

**Files:**
- Create: `internal/entity/training_catalog.go`
- Create: `internal/model/training_catalog.go`
- Create: `internal/repository/training_catalog_repository.go`
- Create: `internal/repository/persistence/training_catalog_postgres.go`
- Create: catalog tests under entity/model/repository/persistence/usecase as needed

- [ ] Define nullable catalog fields and enum-like status values.
- [ ] Define create/update/status/list input and response types.
- [ ] Implement create, update, status update, soft delete, find by public ID, and list.
- [ ] Support filters: `name`, `category`, `training_status`, `training_date`, `training_period`, and `speaker` only if contract permits; document exact supported filters.
- [ ] Exclude soft-deleted catalogs from public reads.
- [ ] Use internal catalog ID cursor and `limit + 1`.
- [ ] Ensure repository mutation paths return committed rows without unsafe post-commit rereads.
- [ ] Keep admin authorization in usecase/route boundary, not request-supplied role.

### Task 3: Enrollment entity, model, repository, and service

**Files:**
- Create: `internal/entity/training_enrollment.go`
- Create: `internal/model/training_enrollment.go`
- Create: `internal/repository/training_enrollment_repository.go`
- Create: `internal/repository/persistence/training_enrollment_postgres.go`
- Create: enrollment tests under entity/model/repository/persistence/usecase as needed

- [ ] Implement transactional self-enrollment using authenticated user ID.
- [ ] Lock catalog row with `SELECT ... FOR UPDATE` before capacity check.
- [ ] Reject deleted/inactive/unavailable catalogs and full capacity with documented status errors.
- [ ] Reject duplicate active enrollment with `409 Conflict`.
- [ ] Implement cancellation as soft delete of member’s active enrollment.
- [ ] Allow new active enrollment after cancellation while preserving history.
- [ ] Implement own history, admin all-history, and admin catalog-history queries.
- [ ] Use internal enrollment ID cursor with `limit + 1`.
- [ ] Exclude deleted enrollments from active enrollment count but include them in history.

### Task 4: Usecases and authorization

**Files:**
- Create: `internal/usecase/training_catalog_usecase.go`
- Create: `internal/usecase/training_enrollment_usecase.go`
- Modify: existing auth identity integration only as needed

- [ ] Permit public catalog list/find without actor identity.
- [ ] Require current DB admin role for catalog create/update/status/delete.
- [ ] Require authenticated identity for enrollment operations.
- [ ] Scope member history/cancellation to current user.
- [ ] Permit admin all-enrollment and catalog-enrollment history.
- [ ] Validate slots, dates, link format, required/nullable values, and enum values.
- [ ] Normalize and trim inputs; keep timestamp and IDs server-controlled.

### Task 5: Handlers, routes, and wiring

**Files:**
- Create: `internal/delivery/http/handler/training_catalog_handler.go`
- Create: `internal/delivery/http/handler/training_enrollment_handler.go`
- Modify: `internal/delivery/http/route/route.go`
- Modify: `internal/config/bootstrap.go`
- Add handler and route tests

- [ ] Add public catalog routes:

```text
GET /training-catalog
GET /training-catalog/{publicID}
```

- [ ] Add admin catalog routes:

```text
POST   /training-catalog
PATCH  /training-catalog/{publicID}
PATCH  /training-catalog/{publicID}/status
DELETE /training-catalog/{publicID}
```

- [ ] Add authenticated enrollment routes:

```text
POST   /training-enrollments
DELETE /training-enrollments/{publicID}
GET    /training-enrollments/my
GET    /training-enrollments
GET    /training-enrollments/catalog/{catalogPublicID}
```

- [ ] Apply no auth middleware to catalog GET routes.
- [ ] Apply admin middleware to catalog mutations and admin enrollment queries.
- [ ] Apply authentication to member enrollment routes.
- [ ] Reuse shared JSON, cursor, limit, and error helpers.
- [ ] Map errors to consistent 400/401/403/404/409/500 responses.

### Task 6: Tests and documentation

**Files:**
- Modify: `api/api-contract.md`, `README.md`, `AGENTS.md`, `CHANGELOG.md`
- Add/update all catalog/enrollment tests

- [ ] Test exact schema, enum reuse, constraints, indexes, and rollback.
- [ ] Test public catalog reads without auth.
- [ ] Test admin-only catalog writes.
- [ ] Test member enrollment, duplicate rejection, cancellation, re-enrollment, and own history.
- [ ] Test admin all-history and catalog-history access.
- [ ] Test capacity and concurrent enrollment never exceeding slots.
- [ ] Test filters, nullable fields, soft delete, and cursor pagination.
- [ ] Document route permissions, cancellation history, re-enrollment, capacity, filters, indexes, and migration version.

### Task 7: Final verification

- [ ] Run `gofmt -w .`.
- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./...`.
- [ ] Run PostgreSQL integration tests with `TEST_POSTGRES_DSN`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Review duplicate code, unused code, SQL transaction boundaries, authorization, route policy, indexes, and docs consistency.
