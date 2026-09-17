# Training Review Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve actionable PR #4 review findings while keeping enrollment lifecycle history based on `created_at` and `deleted_at`.

**Architecture:** Reject empty catalog PATCH requests before repository mutation. Extract shared catalog row scanning for joined enrollment queries to prevent drift. Keep enrollment audit events out of scope because enrollment `created_at` and cancellation `deleted_at` already provide required history. Evaluate indexes without speculative removal.

**Tech Stack:** Go, PostgreSQL, `database/sql`, existing catalog/enrollment tests.

**Spec:** `docs/pr-review/pr-4-review.md`

## Global Constraints

- Catalog reads remain public; catalog writes remain admin-only.
- Enrollment history uses `created_at`; cancellation uses `deleted_at`.
- Do not add enrollment audit tables.
- Preserve capacity locking, soft-delete cancellation, re-enrollment, filters, and cursor pagination.
- No duplicate production scan logic after refactor.

### Task 1: Reject empty catalog PATCH

**Files:**
- Modify: `internal/usecase/training_catalog_usecase.go`
- Modify: `internal/delivery/http/handler/training_catalog_handler.go`
- Test: `internal/usecase/training_catalog_test.go`
- Test: `internal/delivery/http/handler/training_catalog_handler_test.go`

- [ ] Add failing tests for `{}` PATCH returning `400 Bad Request` and not invoking repository update.
- [ ] Run focused tests and verify failure.
- [ ] Return a dedicated invalid-input error when no update fields are supplied.
- [ ] Preserve explicit zero/false/null semantics for fields that are valid update values.
- [ ] Run focused tests and verify pass.

### Task 2: Deduplicate catalog scanning

**Files:**
- Modify: `internal/repository/persistence/training_catalog_postgres.go`
- Modify: `internal/repository/persistence/training_enrollment_postgres.go`
- Modify: repository tests

- [ ] Add a shared scanner for catalog columns used by both direct catalog queries and joined enrollment queries.
- [ ] Replace duplicated field mapping in `scanTrainingEnrollmentJoined` with shared scanner.
- [ ] Preserve nullable fields, status, date, period, speaker, link, and turnover-independent catalog data.
- [ ] Run repository tests and verify unchanged result mapping.

### Task 3: Document deliberate audit/index decisions

**Files:**
- Modify: `docs/pr-review/pr-4-review.md`
- Modify: `api/api-contract.md` or `README.md` only if needed

- [ ] Record that enrollment lifecycle history intentionally uses enrollment `created_at` and cancellation `deleted_at`; no separate enrollment audit event is added.
- [ ] Record that redundant index removal is deferred until query-plan/workload evidence exists.
- [ ] Do not add new runtime behavior for these decisions.

### Task 4: Final verification

- [ ] Run `gofmt -w .` or changed Go files without introducing unrelated formatting churn.
- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Inspect diff for duplicate/unused code and scope.
