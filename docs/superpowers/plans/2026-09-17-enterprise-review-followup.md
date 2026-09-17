# Enterprise Review Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address PR #3 review findings for enterprise update concurrency, database turnover integrity, mutation response consistency, and test coverage.

**Architecture:** Keep existing enterprise layers. Move enterprise mutation read/update/audit behavior into one repository transaction with row locking and `RETURNING`, add database-enforced turnover checks through a follow-up migration, and extend unit/integration tests. Do not change approved API permissions or schema enums.

**Tech Stack:** Go, PostgreSQL, `database/sql`, existing repository/usecase/handler tests.

**Spec:** `docs/pr-review/pr-3-review.md`

## Global Constraints

- Preserve owner/admin authorization and field restrictions.
- Preserve exact requested enterprise enum values.
- Turnover remains `DECIMAL(15,2)` and JSON string end-to-end.
- Mutations and audit inserts remain atomic.
- SQL remains parameterized and context-aware.
- Do not add duplicate helpers or unused exported APIs.

### Task 1: Add database turnover constraints

**Files:**
- Create: `db/migrations/000004_add_enterprise_turnover_checks.up.sql`
- Create: `db/migrations/000004_add_enterprise_turnover_checks.down.sql`
- Modify: `internal/repository/persistence/enterprise_test.go`

- [ ] Add checks enforcing both turnover columns are non-negative and less than `10000000000000`.
- [ ] Make down migration drop both named constraints.
- [ ] Add gated integration assertions rejecting negative and overflow turnover values while accepting valid boundary values.
- [ ] Run migration up/down and focused persistence tests.

### Task 2: Make enterprise update atomic and race-safe

**Files:**
- Modify: `internal/repository/enterprise_repository.go`
- Modify: `internal/repository/persistence/enterprise_postgres.go`
- Modify: `internal/usecase/enterprise_usecase.go`
- Modify: `internal/usecase/enterprise_test.go`
- Modify: `internal/repository/persistence/enterprise_test.go`

- [ ] Add repository update contract that receives actor scope and changed fields, or otherwise preserves authorization before mutation.
- [ ] In one transaction, select target enterprise with `FOR UPDATE` using owner/admin scope.
- [ ] Compare locked current row with requested values.
- [ ] Update only permitted fields while preserving untouched fields.
- [ ] Use `UPDATE ... RETURNING` to obtain final enterprise row without post-commit re-read.
- [ ] Insert audit event in same transaction using actual changed fields.
- [ ] Return committed row after transaction succeeds.
- [ ] Add concurrent update test proving transactions do not silently overwrite stale snapshots and audit fields match committed changes.
- [ ] Run focused usecase and persistence tests.

### Task 3: Return mutation rows without post-commit re-read

**Files:**
- Modify: `internal/repository/persistence/enterprise_postgres.go`
- Modify: repository tests and handler tests as needed

- [ ] Change create and update mutation paths to scan complete row from `RETURNING` inside transaction.
- [ ] Keep audit insertion before commit.
- [ ] Ensure create/update success is not converted to 500 because of a second query after commit.
- [ ] Add repository test covering returned nullable fields and turnover strings.
- [ ] Keep delete response/status behavior unchanged.

### Task 4: Complete review test coverage and minor cleanup

**Files:**
- Modify: `internal/usecase/enterprise_test.go`
- Modify: `internal/repository/persistence/enterprise_test.go`
- Modify: `internal/repository/enterprise_repository.go`
- Modify: `internal/usecase/enterprise_usecase.go` only if justified

- [ ] Add turnover precision round-trip cases such as `0.5` and `0.00`.
- [ ] Add/update inactive or direct-usecase actor guard coverage only if behavior remains intentional.
- [ ] Fix repository interface comment formatting.
- [ ] Do not remove defensive authorization unless direct usecase calls are explicitly safe without it.
- [ ] Do not trim indexes without query evidence; document current index decision if needed.

### Task 5: Documentation and verification

**Files:**
- Modify: `api/api-contract.md`, `README.md`, `AGENTS.md` only if behavior or migration requirements change.

- [ ] Document database turnover checks and migration version.
- [ ] Document update atomicity/audit guarantees if not already stated.
- [ ] Run `gofmt -w .`.
- [ ] Run `go test ./...`.
- [ ] Run `go test -race ./...`.
- [ ] Run PostgreSQL integration tests with `TEST_POSTGRES_DSN` when available.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Inspect diff for duplicate code, unused code, migration rollback correctness, and scope.
