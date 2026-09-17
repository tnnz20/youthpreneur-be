# PR #4 Review: Training Catalog and Enrollment

## Verification

- `go test -count=1 ./...`: passed.
- `go test -race -count=1 ./...`: passed.
- `go vet ./...`: clean.
- `gofmt -l .`: clean.
- `git diff --check`: clean.

## Checklist

- Public catalog reads; admin-only catalog writes: pass.
- Authenticated enrollment operations: pass.
- Member self-scope and admin enrollment scope: pass.
- `user_id` derived from current authenticated identity: pass.
- Capacity protection uses catalog `SELECT ... FOR UPDATE`: pass.
- Duplicate active enrollment protected by partial unique index: pass.
- Cancellation preserves history through `deleted_at`: pass.
- Re-enrollment after cancellation: pass.
- Completed/unset catalog status blocks enrollment: pass.
- Exact schema and reuse of `process_status_enum`: pass.
- Filters and internal-ID cursor pagination: pass.
- Nullable field mapping: pass.
- Parameterized context-aware SQL: pass.
- Migration rollback, indexes, tests, and docs: pass.

## Critical and High Findings

None.

## Medium Findings

### M1. PATCH with empty body updates timestamp — resolved

Location: `internal/repository/persistence/training_catalog_postgres.go:174-200`

An empty PATCH body could update `updated_at` without changing any field or
creating meaningful audit data.

**Resolution:** `UpdateTrainingCatalog` now returns a dedicated
`400 Bad Request` (`usecase.ErrBadRequest`) when no update field is supplied and
never reaches the repository, so no timestamp-only update is written. Explicit
clears such as `{"description": ""}` remain valid updates because the field is
present even when its value is empty; only the absence of every field is
rejected.

### M2. Enrollment lifecycle has no audit events — documented, intentionally skipped

Enrollment and cancellation writes do not have an audit trail equivalent to
enterprise mutations.

**Decision:** Enrollment lifecycle history intentionally relies on the
enrollment row's `created_at` (enrollment) and `deleted_at` (cancellation),
which are preserved on soft-delete and exposed by member and admin history
reads. No separate enrollment audit event table is added; adding one would
duplicate that history and expand the schema without a confirmed operational or
compliance requirement. Revisit only if an audit requirement is confirmed.

### M3. Redundant enrollment indexes — deferred

Migration contains indexes potentially dominated by the partial unique index:

- `idx_training_enrollments_catalog_active`
- `idx_training_enrollments_deleted_at`

**Decision:** Removal is deferred until query-plan and workload evidence exists.
No migration is changed now; dropping an index without evidence risks a query
regression for capacity counting, duplicate checks, or history reads.


## Low Findings

- `dateOnly` uses UTC; local populations near midnight can receive an unexpected `register_date`. Consider configured timezone later.
- Catalog field scanning was duplicated between `scanTrainingCatalog` and `scanTrainingEnrollmentJoined`. Resolved: both now share `trainingCatalogRow` / `toCatalog`, locked by `TestTrainingCatalogScanSharedAcrossJoinedEnrollment`.
- Catalog soft delete does not lock the catalog row, so concurrent enrollment can commit before deletion. Current behavior is acceptable but should be documented if deleted-catalog enrollment visibility matters.
- No dedicated integration test for every capacity authorization branch; existing handler/usecase coverage is strong.

## Duplicate and Unused Code

The catalog scanning duplication across training catalog and joined enrollment repository code is resolved: both paths share `trainingCatalogRow` and its `toCatalog` conversion. No obvious unused production training code or dead exported API found. Shared cursor, limit, identity, and response helpers are reused correctly.

## Conclusion

PR #4 correctly implements public training catalog browsing, admin catalog management, authenticated enrollment, capacity enforcement, cancellation history, re-enrollment, filtering, pagination, migration constraints, and documentation. No critical or high security issues found. Follow-up M1 is resolved, M2 is documented as intentionally relying on enrollment `created_at`/`deleted_at`, M3 is deferred pending query-plan evidence, and the scanning duplication is removed.

## Merge Recommendation

**Approve.** The review follow-up is complete: empty PATCH is rejected, scanning is deduplicated, and the audit/index decisions are documented. No merge blockers.
