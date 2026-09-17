# youthpreneur-be

Go HTTP service for Youthpreneur.

## Getting Started

### Requirements

- Go 1.27+
- Make
- Podman or Docker with Compose support

### Run locally

Start PostgreSQL, apply migrations, then start the API:

```bash
cp .env.example .env
make compose-up
make migrate-up
go run ./cmd/web
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Response:

```json
{"status":"ok"}
```

### Run PostgreSQL

PostgreSQL is provided for local development and the application connects to it
through `POSTGRES_*` settings.

```bash
make compose-up
```

Use Docker instead of Podman:

```bash
make compose-up engine=docker
```

Stop containers:

```bash
make compose-down
```

Stop containers and remove volumes:

```bash
make compose-down-v
```

## Configuration

Configuration is read from environment variables through Viper.

| Variable | Default | Description |
| --- | --- | --- |
| `APP_ADDR` | `:8080` | HTTP listen address |
| `APP_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error` |
| `APP_ENVIRONMENT` | `development` | Application environment |
| `APP_VERSION` | `dev` | Application version reported at startup |
| `APP_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout for `SIGINT`/`SIGTERM` |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `youthpreneur` | PostgreSQL database |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL SSL mode |

`.env` is local-only and ignored by Git. Use `.env.example` as the starting template.

## Migrations

Migrations live in `db/migrations/` and are embedded in `cmd/migrate`, so they
resolve regardless of the working directory.

```bash
make migrate-up                 # apply all pending migrations
make migrate-down               # roll back all migrations
make migrate-version            # print the current version
make migrate-force version=1    # mark version 1 without running it
```

Equivalent direct commands:

```bash
go run ./cmd/migrate up
go run ./cmd/migrate down
go run ./cmd/migrate force 1
go run ./cmd/migrate version
```

`up` and `down` are safe to repeat. A run with nothing to apply reports that no
change occurred. `force` recovers a database left dirty by a failed migration.

See [api/api-contract.md](api/api-contract.md) for database-backed API behavior,
request fields, responses, pagination, and safety limitations.

## Project Layout

```text
cmd/web/                         HTTP server entrypoint
cmd/migrate/                     database migration CLI
db/migrations/                   embedded SQL migrations
internal/config/                 configuration, logger, and dependency bootstrap
internal/delivery/http/          HTTP handlers and routes
internal/entity/                 domain entities
internal/model/                  HTTP request and response models
internal/repository/             repository interfaces
internal/repository/persistence/ PostgreSQL repository implementations
internal/usecase/                application use cases
api/api-contract.md              HTTP API contract
compose.yaml                     local PostgreSQL container
Dockerfile                       API container build
Makefile                         development commands
```

Persistence uses the standard library `database/sql` with the `pgx` driver. No
ORM such as GORM is used.

Request flow:

```text
HTTP request → route → handler → usecase → repository
```

## Commands

```bash
make run
make test
make vet
make fmt
make build
make migrate-up
make migrate-down
make migrate-force version=1
make migrate-version
make compose-up
make compose-down
make compose-down-v
```

## Docker Build

```bash
docker build -t youthpreneur-be .
docker run --rm -p 8080:8080 youthpreneur-be
```

## License

No license has been declared yet.
