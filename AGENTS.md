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
- User creation always assigns `member`; role is not accepted in create request. Admin provisioning requires future authenticated authorization.
- Public IDs use `YTP-` plus six digits and are lookup handles, not secrets.
- User/profile deletion is soft delete and updates both rows in one transaction.
- Cursor pagination uses `users.id`; do not replace it with offset pagination.
- Password hashes are never returned. Password reset is currently unauthenticated and unsafe; keep it restricted to trusted development networks until admin auth middleware exists.
- Request JSON bodies are capped at 1 MiB; password length is 8–72 bytes.

## Change Hygiene

- Preserve file naming: `*_handler.go`, `*_usecase.go`, `*_repository.go`; PostgreSQL implementations belong under `internal/repository/persistence`.
- Use parameterized SQL and keep repository transactions explicit for multi-table operations.
- Do not add authentication or middleware implicitly; document and update `api/api-contract.md` when that boundary changes.
