# Enterprises Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add owned enterprise CRUD with enum filters, cursor pagination, role-based field updates, soft delete, and transactional audit events.

**Architecture:** Add enterprises as an independent aggregate linked one-to-many to users through `enterprises.user_id`. Reuse existing route → handler → usecase → repository layers, current DB identity, internal ID cursors, and PostgreSQL migrations. Write audit events in the same transaction as create, update, and delete.

**Tech Stack:** Go, `database/sql`, PostgreSQL, existing Viper/auth middleware, standard library HTTP, existing migration tooling.

**Spec:** User-approved enterprise requirements in conversation.

## Global Constraints

- Any authenticated member can create many enterprises.
- Member list scope is `user_id = authenticated user ID`; admin list scope has no owner restriction.
- Owner may update only `name`, `business_sector`, `initial_turnover`, and `current_turnover`.
- Admin may update all enterprise fields.
- Owner or admin may soft-delete; `user_id` never comes from request body/query.
- Supported list filters exclude name, turnover, IDs, and timestamps.
- Cursor uses internal `enterprises.id`; default limit 20, maximum 100.
- All SQL parameterized; mutation plus audit insert is one transaction.
- Nullable DB values map to pointers/entities and explicit JSON null where applicable.

### Task 1: Branch and migration

**Files:**
- Create: `db/migrations/000003_create_enterprises.up.sql`
- Create: `db/migrations/000003_create_enterprises.down.sql`

- [ ] Add all requested PostgreSQL enum types.
- [ ] Add `enterprises` with `user_id` FK, turnover decimals, epoch timestamps, and soft delete.
- [ ] Add requested column comments.
- [ ] Add indexes for `user_id`, `district`, `status`, `business_sector`, `deleted_at`, and cursor-friendly owner/deleted combinations.
- [ ] Add `enterprise_audit_events` with actor, enterprise, action, changed fields JSONB, timestamp, FKs, and lookup indexes.
- [ ] Make down migration remove audit table, enterprise table, and enum types in dependency-safe order.

### Task 2: Entity, model, and repository contracts

**Files:**
- Create: `internal/entity/enterprise.go`
- Create: `internal/model/enterprise.go`
- Create: `internal/repository/enterprise_repository.go`

- [ ] Define enum-like constants matching DB values.
- [ ] Define enterprise entity with internal ID, generated public ID, owner ID, nullable fields, turnover decimal representation, timestamps, and deleted state.
- [ ] Define create/update/list request and response types.
- [ ] Define filter with all supported enum/status/district fields, cursor, limit, owner scope, and admin scope.
- [ ] Define repository methods for transactional create/update/delete, find public ID, and list.
- [ ] Keep audit insertion internal to mutation repository transactions.

### Task 3: PostgreSQL repository

**Files:**
- Create: `internal/repository/persistence/enterprise_postgres.go`
- Create/modify: `internal/repository/persistence/enterprise_test.go`

- [ ] Implement nullable scanning with `sql.NullString`, `sql.NullInt64`, and suitable decimal handling.
- [ ] Implement create with server-generated public ID and audit event in one transaction.
- [ ] Implement find by public ID with owner/admin scope and deleted exclusion.
- [ ] Implement list with owner scope, all supported filters, `id > cursor`, `ORDER BY id ASC`, and `limit + 1` behavior.
- [ ] Implement update with allowed field set selected by usecase and audit changed fields in same transaction.
- [ ] Implement soft delete and audit event in same transaction.
- [ ] Map not-found and database errors consistently with existing repository patterns.
- [ ] Add integration tests gated by `TEST_POSTGRES_DSN` for CRUD, ownership, filters, indexes, pagination, soft delete, and audit atomicity.

### Task 4: Enterprise usecase

**Files:**
- Create: `internal/usecase/enterprise_usecase.go`
- Create/modify: `internal/usecase/enterprise_test.go`

- [ ] Read authenticated identity from existing middleware context at handler boundary or pass typed actor identity into usecase.
- [ ] Create enterprise for any authenticated member/admin using authenticated user ID.
- [ ] List owner-owned enterprises for members and all enterprises for admins.
- [ ] Find only owner-owned enterprise for members; admins may find any.
- [ ] Restrict owner updates to name, business sector, and both turnover fields.
- [ ] Allow admin updates to all fields.
- [ ] Allow owner/admin soft delete according to scope.
- [ ] Validate required name, enum values, district length, turnover non-negative/precision bounds, and nullable values.
- [ ] Normalize trimmed strings and ensure public ID/timestamps are server-controlled.
- [ ] Add tests for all permissions, one-to-many ownership, validation, filters, and changed-field audit input.

### Task 5: HTTP handlers and routes

**Files:**
- Create: `internal/delivery/http/handler/enterprise_handler.go`
- Create/modify: `internal/delivery/http/handler/enterprise_handler_test.go`
- Modify: `internal/delivery/http/route/route.go`
- Modify: route tests

- [ ] Add `POST /enterprises` with authentication.
- [ ] Add `GET /enterprises` with authentication and owner/admin scope.
- [ ] Add `GET /enterprises/{publicID}` with authentication and owner/admin scope.
- [ ] Add `PATCH /enterprises/{publicID}` with authentication and owner/admin field policy.
- [ ] Add `DELETE /enterprises/{publicID}` with authentication and owner/admin scope.
- [ ] Do not add `/users/{publicID}/enterprises`.
- [ ] Reuse shared JSON decode/response/error helpers and existing status conventions.
- [ ] Parse filters, cursor, and limit at transport layer; return consistent 400/401/403/404/500 errors.
- [ ] Test request parsing, response nulls, pagination, protected routes, owner restrictions, and admin access.

### Task 6: Wiring and documentation

**Files:**
- Modify: `internal/config/bootstrap.go`
- Modify: `api/api-contract.md`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `CHANGELOG.md`

- [ ] Wire enterprise repository, usecase, handler, and routes through composition root.
- [ ] Document enterprise schema, routes, ownership, admin permissions, filters, cursor contract, soft delete, and audit behavior.
- [ ] Document one-to-many user ownership and server-derived `user_id`.
- [ ] Document indexes and migration version.

### Task 7: Final verification

- [ ] Run `gofmt -w .`.
- [ ] Run `go test ./...`.
- [ ] Run PostgreSQL integration tests with `TEST_POSTGRES_DSN` when available.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Review duplicate code, unused code, authorization, migration rollback, index coverage, audit transaction boundaries, and docs consistency.
