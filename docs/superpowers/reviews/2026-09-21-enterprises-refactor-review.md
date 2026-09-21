# Code Review: Refactor Enterprises Domain (`PR #7`)

**PR:** [#7 - refactor(enterprises): update schema, public ID prefix to TPN, and add public discovery route](https://github.com/tnnz20/youthpreneur-be/pull/7)  
**Date:** 2026-09-21  
**Branch:** `refactor/enterprises`  
**Base:** `master`  
**Commits Reviewed:**
- `7daf6ea` — `feat(db): add migration to refactor enterprises schema and TPN prefix`
- `acd5ecc` — `refactor(enterprises): update entity, repository, and usecase for TPN prefix and new fields`
- `e479904` — `feat(enterprises): add public route and search filter to handlers`
- `6b2ce90` — `docs(enterprises): align api contract and runtime constraints`

---

## Verification Performed

- `go test -count=1 ./...` — **PASS** (all unit and handler tests pass across all packages)
- `go vet ./...` — **PASS** (no warnings or issues)
- `git diff --check` — **PASS** (no trailing whitespace or conflict markers)
- `gofmt -l .` — **PASS** (all modified files properly formatted)

---

## Executive Summary & Conclusion

**Verdict: Ready to Merge (High Quality, Production Ready)**

PR #7 refactors the enterprises aggregate across all layers: schema migrations, entity definitions, repository queries, usecase validations, HTTP delivery, and contract documentation. The implementation strictly adheres to the clean architecture conventions (`handler → usecase → repository → database`) established in `AGENTS.md`.

No critical or important bugs were found. No memory leaks, race conditions, or unescaped SQL queries exist. Row-level locking (`FOR UPDATE OF e`) ensures race-free updates and audit logging without locking joined user tables.

---

## Detailed Code Analysis

### 1. Database Migration (`000007`)
- **Up Migration (`000007_refactor_enterprises_table.up.sql`)**:
  - Safely renames `name` to `enterprise_name`.
  - Backfills existing nulls (`UPDATE enterprises SET enterprise_name = 'Enterprise ' || id WHERE enterprise_name IS NULL;`) before setting `NOT NULL` to prevent migration failure on existing databases.
  - Adds `description TEXT`, `address TEXT`, `focus_commodity VARCHAR(255)`, and `dispora_support VARCHAR(255)`.
  - Column comments accurately describe field purposes and constraints.
- **Down Migration (`000007_refactor_enterprises_table.down.sql`)**:
  - Drops newly added columns, removes `NOT NULL` constraint, and renames `enterprise_name` back to `name`.
  - Reversible and safe.

### 2. Domain & Entity Layer
- `Enterprise` struct updated with `EnterpriseName`, `Description`, `Address`, `FocusCommodity`, `DisporaSupport`, `UserPublicID`, and `OwnerFullName`.
- `PublicEnterprise` and `PublicEnterpriseFilter` cleanly separate the public showcase view from the full aggregate.
- Public ID generation `GenerateEnterprisePublicID()` creates `TPN-` prefixed identifiers using `crypto/rand`, matching the existing `YTP-` entropy design (10^6 space, 5 collision retries).

### 3. Repository & Data Access Layer
- **Atomic Mutation & Owner Data Retrieval**:
  - `CreateEnterprise` and `UpdateEnterprise` utilize CTEs (`WITH inserted AS (...) SELECT ... JOIN users u LEFT JOIN user_profiles p`) to return the mutated enterprise along with owner details in a single round-trip without requiring post-commit queries.
  - `UpdateEnterprise` uses `FOR UPDATE OF e` to only acquire row locks on the `enterprises` table, preventing unnecessary lock contention on `users` or `user_profiles`.
- **Query & Filter Construction**:
  - Both `findEnterprisesQuery` and `findPublicEnterprisesQuery` use parameterized SQL `$1...$N`.
  - Added case-insensitive keyword search (`ILIKE '%' || $N || '%'`) across both `e.enterprise_name` and owner `p.full_name`.
  - Both queries use newest-first ordering (`ORDER BY e.id DESC`) with cursor pagination (`e.id < cursor`).
  - `findPublicEnterprisesQuery` filters `u.deleted_at IS NULL AND u.is_active = TRUE` to ensure enterprises owned by deactivated or soft-deleted users are never surfaced publicly.

### 4. Usecase Layer
- Validation is thorough:
  - `enterprise_name` is trimmed and required (cannot be empty, max 255 chars).
  - `focus_commodity` and `dispora_support` are checked against max 255 chars.
  - Business sector and assessment enums are normalized and validated against enum sets.
- Ownership and permissions:
  - Owners can update `enterprise_name`, `business_sector`, `district`, `description`, `address`, `focus_commodity`, `initial_turnover`, and `current_turnover`.
  - `ownerRestrictedChange()` strictly rejects owner attempts to mutate `dispora_support`, `status`, or assessment enums with `403 Forbidden`.
- Pagination:
  - Default limit for `FindPublicEnterprises` is 9 via `defaultPublicEnterpriseLimit = 9`.
  - Default limit for authenticated `FindEnterprises` remains 20 via `clampLimit`.
  - Max limit is capped at 100 in both.

### 5. HTTP Delivery & Routing
- Handlers properly map query parameters (`search`, `q`, `cursor`, `limit`, and enum filters).
- Status codes correctly follow REST standards: `200 OK`, `201 Created`, `204 No Content`, `400 Bad Request`, `401 Unauthorized`, `403 Forbidden`, `404 Not Found`.
- Route registration: `GET /enterprises/public` is registered without authentication middleware; all other enterprise endpoints remain secured under `Authenticate`.

---

## Duplication and Dead Code Review

| Check | Finding | Status |
| --- | --- | --- |
| **Unused Constants & Variables** | `enterprisePublicIDPrefix`, `defaultPublicEnterpriseLimit`, and length limits are all actively used. | Clean |
| **Unused Struct Fields** | All fields on `Enterprise`, `PublicEnterprise`, `EnterpriseFilter`, `PublicEnterpriseFilter`, and models are populated and serialized. | Clean |
| **Dead Code / Unreachable Branches** | All branches in `ownerRestrictedChange`, `clampPublicEnterpriseLimit`, and `buildFilter` are covered by tests. | Clean |
| **Code Duplication** | Minor similarity between `GenerateEnterprisePublicID()` and `GeneratePublicID()`. Keeping them separate preserves package boundaries and avoids cross-package coupling. | Acceptable |
| **Shared Helpers** | `optionalString`, `parseCursor`, and `parseLimit` are properly reused across package `handler`. | Clean |

---

## Findings

### Critical
*None.*

### Important
*None.*

### Minor
1. **Full Name serialization when user profile is missing**:
   - In `model.EnterpriseResponse` and `model.PublicEnterpriseResponse`, `FullName` is typed as `string` (not `*string`).
   - If an owner does not have a profile, it serializes as `"full_name": ""` rather than `"full_name": null`.
   - *Impact:* Minimal, because standard user registration requires a non-empty `full_name`.

2. **Index recommendation for search at scale**:
   - `e.enterprise_name ILIKE '%' || $N || '%'` uses a leading wildcard, which cannot use standard B-Tree indexes.
   - *Recommendation:* When the enterprises table scales significantly (e.g., >50k rows), consider adding a PostgreSQL `pg_trgm` GIN index on `enterprise_name`. For current application volume, B-tree indexes on `(district, status)` and `id` cursor are sufficient.

---

## Checklist Assessment

| Area | Result |
| --- | --- |
| **Clean Architecture** | Follows `route -> handler -> usecase -> repository -> db`. No layer violations. |
| **SQL Injection Safety** | 100% parameterized SQL. No string interpolation of query values. |
| **Concurrency & Locking** | Atomic transaction with `FOR UPDATE OF e` ensures safe partial updates and audit logging. |
| **Soft Delete & Visibility** | Soft delete properly sets `deleted_at`; public route hides inactive or deleted accounts. |
| **Pagination & Sorting** | Consistent cursor pagination (`e.id < cursor`) with newest-first ordering (`ORDER BY e.id DESC`). |
| **API Contract & Specs** | Fully synchronized with `api/api-contract.md` and `AGENTS.md`. |
| **Test Coverage** | Unit and handler tests cover validation, authorization, filtering, cursor pagination, and response mapping. |

---

## Recommendation

**Approve and Merge.** The changes are complete, verified, and maintain the standards of the codebase.
