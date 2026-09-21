# Code Review: Enterprise Audit Logs & Response Converters Refactor (`PR #8`)

**PR:** [#8 - feat(enterprises): add enterprise audit logs endpoint and refactor response converters](https://github.com/tnnz20/youthpreneur-be/pull/8)  
**Date:** 2026-09-22  
**Branch:** `feat/enterprise-audit-logs`  
**Base:** `master`  
**Commits Reviewed:**
- `8445de9` — `feat(enterprises): add enterprise audit logs endpoint`
- `116f106` — `docs(enterprises): document enterprise audit logs endpoint`
- `b8905b6` — `refactor(model): move response converters from handlers to model`
- `81155e9` — `docs: document response converter convention in AGENTS.md`

---

## Verification Performed

- `go test -count=1 ./...` — **PASS** (all unit, usecase, and handler tests passing across all packages)
- `go vet ./...` — **PASS** (no warnings or diagnostics reported)
- `git diff --check` — **PASS** (no trailing whitespace or merge artifacts)
- `gofmt -l .` — **PASS** (clean Go formatting across entire repository)

---

## Executive Summary & Conclusion

**Verdict: Approved / Ready to Merge**

PR #8 delivers two distinct objectives cleanly:
1. **Enterprise Audit Logs Endpoint (`GET /enterprises/{publicID}/audit-logs`)**: Exposes historical audit mutations (`create`, `update`, `delete`) with actor metadata (`actor_public_id`, `actor_email`, `actor_name`), mutation action, and field-level diffs. It strictly enforces authorization (owner-scoped for regular members, unscoped for admins) and newest-first cursor pagination (`ORDER BY a.id DESC`).
2. **DTO & Converter Architecture Refactor**: Consolidates all entity-to-response converter functions (`To...Response`, `To...Responses`) from HTTP handlers into `internal/model`, satisfying the project architecture standards defined in `AGENTS.md`.

---

## Code Inspection Findings

### 1. Duplicate Code Check
- **Previously Duplicated Helpers**:
  - `optionalString(value string) *string`: Previously declared separately in `enterprise_handler.go` and `training_catalog_handler.go`. Now consolidated into [`internal/model/helpers.go`](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/model/helpers.go) and reused across `model`.
  - `formatOptionalDate(date *time.Time) *string`: Previously declared in `training_catalog_handler.go` and `training_enrollment_handler.go`. Now consolidated into [`internal/model/helpers.go`](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/model/helpers.go).
- **Minor Duplication (Date Format Constant)**:
  - `const birthDateFormat = "2006-01-02"` is defined in [`internal/model/user.go`](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/model/user.go#L80) and in [`internal/delivery/http/handler/user_handler.go`](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/delivery/http/handler/user_handler.go#L16), while [`internal/model/helpers.go`](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/model/helpers.go#L21) hardcodes `"2006-01-02"`.
  - *Impact:* Very low. All instances use the standard ISO 8601 date layout (`2006-01-02`).
  - *Suggestion:* In a future clean-up pass, define an unexported or exported layout constant (e.g. `const DateFormat = "2006-01-02"`) in `internal/model/helpers.go`.
- **Defensive Map Initialization**:
  - `event.ChangedFields` nil check is performed both in repository row scanning (`if event.ChangedFields == nil { event.ChangedFields = map[string]any{} }`) and in `ToEnterpriseAuditEventResponse`.
  - *Impact:* Harmless redundancy; serves as defensive programming against uninitialized maps serializing to JSON `null`.

### 2. Unused Code & Imports Check
- **Imports**: All unused imports were removed:
  - `"github.com/tnnz20/youthpreneur-be/internal/entity"` was cleaned from `auth_handler.go`.
  - `"time"` was removed from `training_catalog_handler.go`.
  - Handler packages now import `internal/model` for DTO conversions.
- **Dead Code**:
  - Zero unused functions, constants, or variables remain.
  - All converters moved to `internal/model` have active call sites in their respective HTTP handlers.

---

## Detailed Architectural Review

### 1. Enterprise Audit Logs

#### Persistence Layer (`internal/repository/persistence/enterprise_postgres.go`)
- **Query Structure**:
  - Validates enterprise existence and authorization in a primary query:
    ```sql
    SELECT id
    FROM enterprises
    WHERE public_id = $1
      AND deleted_at IS NULL
      AND ($2::int = 0 OR user_id = $2)
    ```
    Correctly treats deleted enterprises as non-existent (`404 Not Found`), matching the behavior of `GetEnterprise`.
  - Fetches audit events joining with `users` and `user_profiles`:
    ```sql
    SELECT
        a.id,
        COALESCE(u.public_id, ''),
        COALESCE(u.email, ''),
        COALESCE(p.full_name, ''),
        a.action,
        a.changed_fields,
        a.created_at
    FROM enterprise_audit_events a
    LEFT JOIN users u ON u.id = a.actor_user_id
    LEFT JOIN user_profiles p ON p.user_id = u.id
    WHERE a.enterprise_id = $1
      AND ($2::int = 0 OR a.id < $2)
    ORDER BY a.id DESC
    LIMIT $3
    ```
  - Use of `LEFT JOIN` and `COALESCE` safely handles scenarios where the actor user was soft-deleted or lacks a profile record.

#### Usecase Layer (`internal/usecase/enterprise_usecase.go`)
- Strictly verifies that actor identity is non-zero (`input.Actor.ID == 0 -> ErrForbidden`).
- Enforces owner scoping (`scopeOwnerID` passes `0` for Admin, `actor.ID` for Member).
- Adopts cursor pagination semantics with `limit + 1` query size, slicing to `limit` and returning `NextCursor = events[limit-1].ID` only when more rows exist.

#### Delivery Layer (`internal/delivery/http/handler/enterprise_handler.go` & `route.go`)
- Registered under authenticated group: `GET /enterprises/{publicID}/audit-logs`.
- Robust parsing for `cursor` and `limit` query parameters with validation error handling (`400 Bad Request`).
- Consistent error mapping via `writeUsecaseError`.

### 2. Model DTO & Converter Refactor

- All converter functions now reside in package `model`:
  - `ToMeResponse` (`internal/model/auth.go`)
  - `ToUserResponse`, `ToUserResponses` (`internal/model/user.go`)
  - `ToEnterpriseResponse`, `ToEnterpriseResponses`, `ToPublicEnterpriseResponse`, `ToPublicEnterpriseResponses`, `ToEnterpriseAuditEventResponse`, `ToEnterpriseAuditEventResponses` (`internal/model/enterprise.go`)
  - `ToTrainingCatalogResponse`, `ToTrainingCatalogResponses` (`internal/model/training_catalog.go`)
  - `ToTrainingEnrollmentResponse`, `ToTrainingEnrollmentResponses` (`internal/model/training_enrollment.go`)
- `internal/model` depends solely on `internal/entity`, avoiding any cyclic dependency with HTTP handlers.

### 3. Documentation & Contracts
- `api/api-contract.md`:
  - Added route permission table entry for `GET /enterprises/{publicID}/audit-logs`.
  - Added detailed endpoint documentation (Section 20), including query parameter table, sample JSON response, and status codes.
  - Successfully renumbered subsequent training catalog and enrollment sections (21 through 31).
- `AGENTS.md`:
  - Added rule in `Structure` explicitly stating that request/response DTOs and `To...Response` converter functions live in `internal/model`, and HTTP handlers must not declare local response converters.

---

## Suggestions & Next Steps

1. **Date Format Unification (Optional Quality of Life)**:
   - Move `birthDateFormat = "2006-01-02"` to an unexported or exported constant in `internal/model/helpers.go` (e.g. `const DateFormat = "2006-01-02"`), and reference it in `formatOptionalDate` and `ToUserResponse`.
2. **Merging**:
   - The branch is clean, fully verified, and ready to be merged into `master`.
