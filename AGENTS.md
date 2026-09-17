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

- Configuration comes from Viper environment variables; copy `.env.example` to `.env`. Never commit `.env` or credentials.
- PostgreSQL timestamps are Unix epoch seconds in `BIGINT`; `birth_date` is `YYYY-MM-DD`.
- User creation always assigns `member`; role is not accepted in create request. Admin provisioning requires a trusted database or admin flow.
- Public IDs use `YTP-` plus six digits and are lookup handles, not secrets.
- User/profile deletion is soft delete and updates both rows in one transaction.
- Cursor pagination uses `users.id`; do not replace it with offset pagination.
- Password hashes are never returned.
- Auth is cookie-based JWT: 15-minute access token, 7-day opaque refresh token stored only as a SHA-256 hash. Cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` only when `APP_ENV=production`.
- `APP_AUTH_SECRET` must be at least 32 bytes outside development; startup rejects weak secrets.
- Refresh rotation revokes the consumed session and issues a replacement in one transaction; logout revokes and clears cookies.
- Request JSON bodies are capped at 1 MiB; password length is 8–72 bytes.

## Change Hygiene

- Preserve file naming: `*_handler.go`, `*_usecase.go`, `*_repository.go`; PostgreSQL implementations belong under `internal/repository/persistence`.
- Use parameterized SQL and keep repository transactions explicit for multi-table operations.
- Authentication, role, ownership, CORS, and rate-limit middleware live in `internal/delivery/http/middleware`; route permissions are defined in `route.go` and must stay aligned with `api/api-contract.md`.
