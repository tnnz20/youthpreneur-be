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

Schema notes:

- `users` and `user_profiles` use `SERIAL` integer primary keys.
- `users.email` and `users.public_id` are unique. The bcrypt hash is stored in
  `users.password` and is never returned by the API.
- `created_at`, `updated_at`, and `deleted_at` are Unix epoch seconds stored as
  `BIGINT`. `deleted_at` is `NULL` until a soft delete.
- User deletes are soft deletes; deleted users are excluded from reads.
- `user_role` is an enum of `admin` or `member`. `user_profiles.gender` is
  `VARCHAR(50)`.
- Public IDs are six random decimal digits (`YTP-DDDDDD`), a 10^6 space. They
  are lookup handles, not authentication credentials, and must not be treated as
  secret. After a bounded number of collisions the create request fails rather
  than looping; widen the format (8+ digits or alphanumeric) before the user
  count approaches that ceiling.
- New users are always created with the `member` role. Any `role` sent to
  `POST /users` is ignored. Admin provisioning needs a trusted, authenticated
  path, which this PR does not add.
- Adding a future role is a two-part change: add the value to the `user_role`
  enum with a new migration, and update application-side role handling for any
  trusted provisioning path.
- `user_profiles` holds `full_name`, `nik`, `birth_date` (`DATE`), `gender`,
  `district`, `phone` (`VARCHAR(32)`), and `address` (`TEXT`). `birth_date` is
  exchanged as `YYYY-MM-DD` (ISO 8601).
- Deleting a user soft deletes both `users` and its `user_profiles` row in one
  transaction, using the same Unix timestamp for `deleted_at` and `updated_at`.

## User API

No authentication or middleware is wired yet. See the limitation below before
exposing these routes.

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/users` | Create a user and profile |
| `GET` | `/users` | List users with cursor pagination |
| `GET` | `/users/{publicID}` | Get one user with profile |
| `DELETE` | `/users/{publicID}` | Soft delete a user |
| `PUT` | `/users/{publicID}/profile` | Replace profile fields |
| `PATCH` | `/users/{publicID}/status` | Set the active flag |
| `PUT` | `/users/{publicID}/password` | Change a password using the current one |
| `POST` | `/users/{publicID}/password/reset` | Set a password without the current one |

Public IDs use the form `YTP-` plus six random digits, for example
`YTP-482910`. Passwords are hashed with bcrypt and never returned in responses.
Request bodies are capped at 1 MiB. Passwords must be 8 to 72 bytes; the upper
bound matches bcrypt's input limit so long passwords are rejected instead of
being silently truncated. `birth_date` must be a valid ISO 8601 date that is not
in the future.

### Cursor pagination

`GET /users` is ordered by `users.id` ascending and never uses offset
pagination.

| Query parameter | Description |
| --- | --- |
| `cursor` | Return users with `id` greater than this value |
| `limit` | Page size, clamped to 1-100; default 20 |
| `district` | Optional exact profile district filter |
| `gender` | Optional `male` or `female` filter |

The response includes `next_cursor` when another page exists. Pass it back as
`cursor` to fetch the next page. The field is omitted on the last page.

```bash
curl "http://localhost:8080/users?limit=2&district=Bandung"
```

```json
{
  "users": [
    {
      "public_id": "YTP-482910",
      "email": "alice@example.com",
      "role": "member",
      "is_active": true,
      "created_at": 1700000000,
      "updated_at": 1700000000,
      "profile": {
        "full_name": "Alice",
        "district": "Bandung",
        "phone": "08123456789",
        "address": "Jalan Mawar 1"
      }
    }
  ],
  "next_cursor": "42"
}
```

### Development limitation

There is no authentication or authorization yet. In particular,
`POST /users/{publicID}/password/reset` resets a password without the current
password. This endpoint is unsafe until authentication middleware is added;
then restrict it to authenticated admins only. Do not expose it outside a
trusted development network before that.

## Project Layout

```text
cmd/web/                         HTTP server entrypoint
cmd/migrate/                     database migration CLI
db/migrations/                   embedded SQL migrations
internal/config/                 configuration, logger, and dependency bootstrap
internal/delivery/http/          HTTP handlers and routes
internal/entity/                 domain entities
internal/model/                  HTTP request and response models
internal/repository/             repository interfaces and implementations
internal/usecase/                application use cases
compose.yaml                     local PostgreSQL container
Dockerfile                       API container build
Makefile                         development commands
```

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
