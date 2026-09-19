# AGENTS.md

## Commands

- `make fmt` formats `cmd` and `internal`; use `gofmt -w .` when changing migration or API files too.
- `make test` runs `go test ./...`; `make vet` runs `go vet ./...`; `make build` builds `bin/web`.
- Run `go test ./...`, `go vet ./...`, and `git diff --check` before claiming a change is ready.
- Integration repository tests run only when `TEST_POSTGRES_DSN` is set and point to a database with migrations applied: `TEST_POSTGRES_DSN='...' go test ./internal/repository/persistence -run TestUserRepositoryIntegration`.
- Start local PostgreSQL with `make compose-up`, apply schema with `make migrate-up`, and stop it with `make compose-down`; use `engine=docker` to replace default Podman.
- Migration CLI targets are `make migrate-up`, `make migrate-down`, `make migrate-version`, and `make migrate-force version=1`.

## Structure

- `cmd/web` starts HTTP server; `cmd/migrate` runs embedded PostgreSQL migrations.
- `internal/config/bootstrap.go` is composition root: opens and pings PostgreSQL, then wires repository → usecase → handler → routes.
- HTTP flow is `internal/delivery/http/route` → `handler` → `usecase` → `repository`; PostgreSQL implementation lives in `internal/repository/persistence`.
- SQL migrations live in `db/migrations` and are embedded by `db/migrations/embed.go`; update the migration CLI import if this path changes.
- `api/api-contract.md` is API contract source; keep route, request, response, status, and safety docs aligned with `route.go` and model tags.

## Runtime Constraints

- Configuration comes from Viper and required `.env`; copy `.env.example` to `.env`. Never commit `.env` or credentials.
- PostgreSQL timestamps are Unix epoch seconds in `BIGINT`; `birth_date` is `YYYY-MM-DD`.
- User creation always assigns `member`; role is not accepted in create request. Admin provisioning requires a trusted database or admin flow, currently `cmd/seeder` driven by `SEEDER_ADMIN_EMAIL` and `SEEDER_ADMIN_PASSWORD`. The seeder reads `SEEDER_ADMIN_EMAIL` and `SEEDER_ADMIN_PASSWORD` from required `.env` only, ignores process environment overrides for these credentials, keeps user/profile creation transactional, and never logs the password or hash.
- `GET /users` lists active member users only and excludes admins before cursor and limit are applied. `GET /users/{publicID}` may return an active admin.
- Public IDs use `YTP-` plus six digits and are lookup handles, not secrets.
- User/profile deletion is soft delete and updates both rows in one transaction.
- Cursor pagination uses `users.id`; do not replace it with offset pagination.
- Password hashes are never returned.
- Auth is cookie-based JWT: 15-minute access token, 7-day opaque refresh token stored only as a SHA-256 hash. Cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` only when `APP_ENV` is exactly `production`.
- `APP_AUTH_SECRET` has no default and is required in every environment; startup rejects a missing or empty secret, and a secret shorter than 32 bytes outside development.
- Refresh rotation revokes the consumed session and issues a replacement in one transaction; replay of a revoked token revokes the user's whole session family; password change, admin reset, and deactivation revoke all user sessions; expired sessions are cleaned opportunistically. Logout revokes and clears cookies.
- Authorization uses the current database role loaded by `Authenticate`, never the JWT role claim, so demotions take effect on the next request.
- Rate limiting is fixed-window, in-memory, and keyed by `RemoteAddr`; it requires direct single-process exposure. Add trusted-proxy IP handling and shared storage before proxied or multi-replica deployments.
- Request JSON bodies are capped at 1 MiB; password length is 8–72 bytes.
- Enterprises are owned one-to-many by users; `enterprises.user_id` is always the authenticated identity and is never accepted from request body or query.
- Enterprise reads are owner-scoped for members and unscoped for admins. Owners update only `name`, `business_sector`, `initial_turnover`, and `current_turnover`; admins may also update `district`, `status`, and the assessment fields (`legal_status`, `business_digitization`, `intervention_needs`, `training_status`, `mentoring_status`, `capital_access`, `partnership`).
- Enterprise list filters are `district`, `status`, `business_sector`, `legal_status`, `business_digitization`, `intervention_needs`, `training_status`, `mentoring_status`, `capital_access`, and `partnership`; name, turnover, IDs, and timestamps are not filterable. Cursor pagination uses internal `enterprises.id` with `limit + 1`.
- Enterprise create, update, and delete write an `enterprise_audit_events` row in the same transaction. Enterprise deletion is soft.
- Enterprise update locks the row with `SELECT ... FOR UPDATE`, merges only the requested fields, derives the actual changed fields for the audit event, and returns the mutated row from `UPDATE ... RETURNING` without a post-commit re-read. Create returns its row from `INSERT ... RETURNING`.
- Enterprise turnovers are `DECIMAL(15,2)` values carried as strings end to end. Migration `000004` adds named `CHECK` constraints (`enterprises_initial_turnover_non_negative`, `enterprises_current_turnover_non_negative`) rejecting negative turnover and values at or above `10000000000000`. Enterprise enums are `business_sector_enum`, `enterprise_status_enum`, `legal_status_enum`, `business_digitization_enum`, `intervention_needs_enum`, `process_status_enum`, and `general_status_enum`; `name` and the assessment enums are nullable.
- Training catalog reads (`GET /training-catalog`, `GET /training-catalog/{publicID}`) are public; catalog create, update, status update, and delete are admin-only. Enrollment routes require authentication; members enroll, cancel, and view only their own enrollments, while admins list all and per-catalog history. `training_enrollments.user_id` is always the authenticated identity and is never accepted from request body or query.
- `training_catalog.training_status` reuses `process_status_enum`; do not add a new enum. A catalog accepts enrollments only while its status is `planned` or `ongoing`. `training_slots` is a positive `INTEGER` or `NULL` for unlimited, enforced by the named `CHECK` `training_catalog_training_slots_positive`.
- Enrollment creation locks the catalog row with `SELECT ... FOR UPDATE`, checks for an existing active enrollment, counts active enrollments, and inserts in one transaction, so capacity is never exceeded. Duplicate active enrollment returns `409 Conflict`; cancellation sets `deleted_at`, preserves history, and allows re-enrollment via the partial unique active index `training_enrollments_active_unique`.
- Training catalog filters are `category`, `training_status`, `training_date`, and `training_period`; name, phone, speaker, link, slots, IDs, and timestamps are not filterable. Cursor pagination uses internal `training_catalog.id` / `training_enrollments.id` with `limit + 1`. Migration `000005` adds both tables and their indexes and does not recreate `process_status_enum`.

## Change Hygiene

- Preserve file naming: `*_handler.go`, `*_usecase.go`, `*_repository.go`; PostgreSQL implementations belong under `internal/repository/persistence`.
- Use parameterized SQL and keep repository transactions explicit for multi-table operations.
- Authentication, role, ownership, CORS, and rate-limit middleware live in `internal/delivery/http/middleware`; route permissions are defined in `route.go` and must stay aligned with `api/api-contract.md`.
