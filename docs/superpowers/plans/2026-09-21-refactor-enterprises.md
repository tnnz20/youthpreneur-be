# Refactor Enterprises Implementation Plan

Refactor the enterprise domain to support rich business profiles, distinct enterprise public identifiers (`TPN-`), owner profile enrichment (`full_name` and `user_public_id` from `users.public_id` and `user_profiles.full_name`), and a dedicated public showcase route.

## User Feedback & Decisions

> [!IMPORTANT]
> **1. Database Column: `public_id` vs `enterprise_public_id` on `enterprises` table**
> - **Recommendation**: Keep **`public_id`** on the `enterprises` table.
> - **Rationale**: In our database schema, every entity table consistently uses `id` and `public_id` (`users.public_id`, `training_catalog.public_id`, `training_enrollments.public_id`). The prefix itself (`TPN-` vs `YTP-`) cleanly distinguishes them.
> - In SQL queries, we qualify as `e.public_id` and `u.public_id`.
> - In JSON responses:
>   - The enterprise's own lookup ID is `"public_id": "TPN-123456"` (or `"enterprise_public_id"` if you prefer, but `"public_id"` is standard for an enterprise resource).
>   - The owner's lookup ID is `"user_public_id": "YTP-000123"` (sourced from `users.public_id`).

> [!IMPORTANT]
> **2. Column Data Types: `dispora_support` & `focus_commodity` (`VARCHAR(255)` vs `TEXT`)**
> - **Recommendation**: Use **`VARCHAR(255)`** for `dispora_support` and `focus_commodity`.
> - **Rationale**: Program titles/grants (e.g. *"Bantuan Modal Usaha 2025"*, *"Fasilitasi Sertifikasi Halal"*) and business focus/commodities (e.g. *"Kopi Robusta & Olahan Kopi"*) are concise descriptors. Setting `VARCHAR(255)` prevents unbounded string dumps and aligns with other bounded attributes (`enterprise_name VARCHAR(255)`, `district VARCHAR(128)`).
> - `description` and `address` remain **`TEXT`** for multi-line freeform content.

> [!IMPORTANT]
> **3. Confirmed: New Columns on `enterprises` (not a new table)**
> - As confirmed, `focus_commodity` (`VARCHAR(255)`) and `dispora_support` (`VARCHAR(255)`) will be added as new columns directly on `enterprises`.

> [!IMPORTANT]
> **4. Field Mutability & Roles (Owner can update `district`)**
> - **Owner (Member)** can update: `enterprise_name`, `business_sector`, `district`, `description`, `address`, `focus_commodity`, `initial_turnover`, `current_turnover`.
> - **Admin** can update all the above plus: `status`, `dispora_support`, and assessment fields (`legal_status`, `business_digitization`, `intervention_needs`, `training_status`, `mentoring_status`, `capital_access`, `partnership`).

> [!IMPORTANT]
> **5. Public ID Format (`TPN-` vs `YTP-`)**
> - Users and training entities retain `YTP-DDDDDD`.
> - Enterprises switch to `TPN-DDDDDD` (short for en**T**er**P**re**N**eur).
> - Public ID generator `GenerateEnterprisePublicID()` will generate `TPN-` followed by 6 random decimal digits.
> - Enterprise path parameters validate against `^TPN-[0-9]{6}$`.

> [!IMPORTANT]
> **6. Public Enterprise Route (`GET /enterprises/public`)**
> - **Route**: `GET /enterprises/public` (Public access, no authentication required).
> - **Response Payload**:
>   - `public_id` (enterprise `TPN-xxxxxx`)
>   - `enterprise_name`
>   - `full_name` (entrepreneur's name from `user_profiles.full_name`)
>   - `business_sector`
>   - `district`
>   - `description`
>   - `focus_commodity`
>   - `dispora_support`
>   - `intervention_needs`
> - Safe: Excludes turnover figures, private address, legal/capital assessment details, and internal IDs.

---

## Proposed Changes

### Git Branch
- Create and switch to branch `refactor/enterprises`.

---

### Database Migration

#### [NEW] [000007_refactor_enterprises_table.up.sql](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/db/migrations/000007_refactor_enterprises_table.up.sql)
```sql
ALTER TABLE enterprises RENAME COLUMN name TO enterprise_name;

UPDATE enterprises SET enterprise_name = 'Enterprise ' || id WHERE enterprise_name IS NULL;
ALTER TABLE enterprises ALTER COLUMN enterprise_name SET NOT NULL;

ALTER TABLE enterprises ADD COLUMN description TEXT;
ALTER TABLE enterprises ADD COLUMN address TEXT;
ALTER TABLE enterprises ADD COLUMN focus_commodity VARCHAR(255);
ALTER TABLE enterprises ADD COLUMN dispora_support VARCHAR(255);

COMMENT ON COLUMN enterprises.enterprise_name IS 'Official name of the enterprise; non-null.';
COMMENT ON COLUMN enterprises.focus_commodity IS 'Primary product line or focus commodity; up to 255 chars.';
COMMENT ON COLUMN enterprises.dispora_support IS 'Dispora program support or grant received; up to 255 chars.';
```

#### [NEW] [000007_refactor_enterprises_table.down.sql](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/db/migrations/000007_refactor_enterprises_table.down.sql)
```sql
ALTER TABLE enterprises DROP COLUMN IF EXISTS dispora_support;
ALTER TABLE enterprises DROP COLUMN IF EXISTS focus_commodity;
ALTER TABLE enterprises DROP COLUMN IF EXISTS address;
ALTER TABLE enterprises DROP COLUMN IF EXISTS description;

ALTER TABLE enterprises ALTER COLUMN enterprise_name DROP NOT NULL;
ALTER TABLE enterprises RENAME COLUMN enterprise_name TO name;
```

---

### Domain Entities & Repositories

#### [MODIFY] [entity/enterprise.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/entity/enterprise.go)
- In `Enterprise` struct:
  - `EnterpriseName string` (replaces `Name string`)
  - `Description string`
  - `Address string`
  - `FocusCommodity string`
  - `DisporaSupport string`
  - `UserPublicID string` (joined from `users.public_id`)
  - `OwnerFullName string` (joined from `user_profiles.full_name`)
- In `EnterpriseUpdate` struct:
  - `EnterpriseName *string`
  - `District *string`
  - `Description *string`
  - `Address *string`
  - `FocusCommodity *string`
  - `DisporaSupport *string`
- Add `PublicEnterprise` struct containing the whitelist of public showcase fields.

#### [MODIFY] [repository/enterprise_repository.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/repository/enterprise_repository.go)
- Add `FindPublicEnterprises(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.PublicEnterprise, error)` to the `EnterpriseRepository` interface.

#### [MODIFY] [repository/persistence/enterprise_postgres.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/repository/persistence/enterprise_postgres.go)
- Update SQL queries to join:
  `LEFT JOIN users u ON u.id = e.user_id LEFT JOIN user_profiles p ON p.user_id = u.id`
  selecting `u.public_id` and `p.full_name`.
- Update `CreateEnterprise`, `FindEnterpriseByPublicID`, `FindEnterprises`, and `UpdateEnterprise` for new columns and joined user fields.
- Implement `FindPublicEnterprises` querying active, non-deleted enterprises.

---

### Usecase Layer

#### [MODIFY] [usecase/enterprise_usecase.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/usecase/enterprise_usecase.go)
- Define `enterprisePublicIDPrefix = "TPN-"`.
- Add `GenerateEnterprisePublicID()` returning `TPN-[0-9]{6}`.
- Enforce `enterprise_name` is non-empty, trimmed, $\le 255$ characters.
- Update `CreateEnterpriseInput` and `UpdateEnterpriseInput` with the new fields.
- Allow owners to update `district`, `enterprise_name`, `business_sector`, `description`, `address`, `focus_commodity`, `initial_turnover`, `current_turnover`.
- Add `ListPublicEnterprises(ctx context.Context, filter entity.EnterpriseFilter) ([]entity.PublicEnterprise, int, error)`.

#### [MODIFY] [usecase/enterprise_test.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/usecase/enterprise_test.go)
- Update mock assertions for `TPN-` prefix, `enterprise_name`, owner district update, new columns, and public enterprise listing.

---

### HTTP Delivery Layer

#### [MODIFY] [model/enterprise.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/model/enterprise.go)
- Update `CreateEnterpriseRequest`:
  - `enterprise_name` (required string)
  - `district`, `description`, `address`, `focus_commodity` (optional strings)
- Update `UpdateEnterpriseRequest`:
  - `enterprise_name`, `district`, `description`, `address`, `focus_commodity`, `dispora_support` (optional string pointers)
- Update `EnterpriseResponse`:
  - `public_id` (`TPN-xxxxxx`)
  - `user_public_id` (`YTP-xxxxxx`, from `users.public_id`)
  - `full_name` (string, from `user_profiles.full_name`)
  - `enterprise_name` (string)
  - `district`, `description`, `address`, `focus_commodity`, `dispora_support`
- Add `PublicEnterpriseResponse`:
  - `public_id`, `enterprise_name`, `full_name`, `business_sector`, `intervention_needs`, `district`, `description`, `focus_commodity`, `dispora_support`.
- Add `PublicEnterpriseListResponse` with `enterprises` and `next_cursor`.

#### [MODIFY] [delivery/http/handler/enterprise_handler.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/delivery/http/handler/enterprise_handler.go)
- Update `publicIDPattern = regexp.MustCompile(`^TPN-[0-9]{6}$`)`.
- Add `ListPublic(w http.ResponseWriter, r *http.Request)`.

#### [MODIFY] [delivery/http/route/route.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/delivery/http/route/route.go)
- Register:
  `rt.register(mux, "GET /enterprises/public", nil, rt.deps.EnterpriseHandler.ListPublic)`

#### [MODIFY] [delivery/http/handler/enterprise_handler_test.go](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/internal/delivery/http/handler/enterprise_handler_test.go)
- Update tests for `TPN-` IDs, `enterprise_name`, new fields, owner district update, and `GET /enterprises/public`.

---

### Documentation & Contract

#### [MODIFY] [api/api-contract.md](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/api/api-contract.md)
- Update enterprise schemas, endpoint permissions table, and example requests/responses.
- Document `GET /enterprises/public`.

#### [MODIFY] [AGENTS.md](file:///c:/Users/tnnz/Documents/projects/freelancer/youthpreneur-be/AGENTS.md)
- Update enterprise constraints: `TPN-` public ID prefix, `enterprise_name`, owner district update permissions, and `GET /enterprises/public`.

---

## Verification Plan

### Automated Tests
- `go test ./...`: Run all unit and mock tests across all packages.
- `go vet ./...`: Run static analysis.
- `git diff --check`: Verify whitespace and git cleanliness.
- `make fmt`: Ensure all Go files conform to `gofmt`.
- Integration tests (when PostgreSQL available):
  `TEST_POSTGRES_DSN='...' go test ./internal/repository/persistence -run TestEnterpriseRepositoryIntegration`

### Manual / API Verification
- Verify `POST /enterprises` generates `TPN-xxxxxx` and persists `enterprise_name`, `district`, `description`, `address`, `focus_commodity`.
- Verify `PATCH /enterprises/{publicID}` allows owner to update `district`.
- Verify `GET /enterprises` and `GET /enterprises/{publicID}` return `user_public_id`, `full_name`, and all enterprise fields.
- Verify `GET /enterprises/public` can be called without any auth cookie and returns only the public whitelist fields.
