# PR #3 Review: `feat/enterprises`

## Scope

Reviewed enterprise schema, migrations, indexes, entities, models, repository, usecase, handlers, routes, ownership, admin authorization, audit events, filters, pagination, tests, duplicate code, unused code, and docs.

## Verification

- `go test -count=1 ./...` passed.
- `go test -race ./internal/usecase ./internal/delivery/http/handler ./internal/repository/persistence` passed.
- `go vet ./...` passed.
- `git diff --check` passed.
- `gofmt -l .` reports pre-existing master files; enterprise files are formatted.

## Implemented Checklist

- CRUD routes and handler wiring: pass.
- Authenticated member creation: pass.
- One-to-many user ownership: pass.
- Server-derived `user_id`: pass; request body/query cannot set ownership.
- Member owner scope and admin global scope: pass.
- Owner update fields restricted to `name`, `business_sector`, `initial_turnover`, and `current_turnover`: pass.
- Admin full update: pass.
- Soft delete and deleted-row exclusion: pass.
- Exact seven requested PostgreSQL enums: pass.
- Nullable fields and JSON `null`: pass.
- `DECIMAL(15,2)` turnover values carried as strings: pass.
- Internal `enterprises.id` cursor with `limit + 1`: pass.
- Requested list filters: pass.
- Create/update/delete audit rows in same transaction: pass.
- Indexes and rollback migration: pass.
- Docs aligned with implementation: pass.
- No obvious duplicate production code or unused enterprise code found.

## Medium Findings

### M1. Update check-then-act race

Locations: `internal/usecase/enterprise_usecase.go:408-562`, `internal/repository/persistence/enterprise_postgres.go:199`

Update reads the existing row, computes changed fields, then performs a full-row update in a separate transaction. Concurrent updates can overwrite each other, and audit `changed_fields` can describe a stale snapshot rather than the committed change.

**Suggestion:** Lock the enterprise row with `SELECT ... FOR UPDATE` and compute/update/audit inside one transaction, or use `UPDATE ... RETURNING` and derive changed fields from the locked prior row.

### M2. Missing DB turnover constraints

Location: `db/migrations/000003_create_enterprises.up.sql:43-44`

Comments describe non-negative turnover, but PostgreSQL does not enforce it. Writers outside this API can store negative values or exceed the intended `DECIMAL(15,2)` range.

**Suggestion:** Add:

```sql
CHECK (initial_turnover >= 0 AND initial_turnover < 10000000000000),
CHECK (current_turnover >= 0 AND current_turnover < 10000000000000)
```

### M3. Mutation succeeds but response re-read can fail

Locations: `internal/repository/persistence/enterprise_postgres.go:117`, `:250`

Create/update commit first, then re-read the row. A re-read failure returns HTTP 500 after the mutation already succeeded.

**Suggestion:** Return the complete row from the mutation transaction using `RETURNING`, or document eventual response behavior and provide idempotent retry handling.

## Low Findings

- `internal/usecase/enterprise_usecase.go:180-182`: defensive `Actor.ID == 0` branch is unreachable through normal authenticated routing; retain only if direct-usecase protection is desired.
- Migration creates many single-column indexes on low-cardinality enum fields in addition to composite/cursor indexes. Consider trimming unused indexes after query-volume evidence exists.
- `internal/repository/enterprise_repository.go:23-24`: ownership scope comment formatting is unclear.
- `enterprise_handler.go:197` and `IdentityFunc` are small indirections; consistent but optional cleanup.
- `gofmt -l .` reports unrelated pre-existing files; consider a separate formatting cleanup outside this PR.

## Test Gaps

- No concurrent enterprise update test proving lost-update behavior and audit correctness.
- No integration test for turnover database constraints.
- Turnover precision coverage is acceptable but could add values such as `0.5` and `0.00` to integration round-trip tests.

## Duplicate and Unused Code

No obvious duplicate production enterprise helpers or unused enterprise implementation found. Existing shared helpers are reused for decoding, cursor/limit parsing, IDs, and validation. Small wrappers noted under Low Findings are not defects.

## Conclusion

PR #3 faithfully implements requested enterprise management: exact schema enums, one-to-many ownership, owner/admin authorization, restricted owner updates, filters, cursor pagination, soft delete, indexes, and transactional audit events. Tests, race checks, vet, and integration migration checks pass.

## Merge Recommendation

**Approve / merge.** M1–M3 are follow-up improvements, not merge blockers. Add turnover `CHECK` constraints before allowing direct database writers or external administrative tooling; address update locking before high-concurrency enterprise editing.
