# youthpreneur-be

Go HTTP service for Youthpreneur.

## Getting Started

### Requirements

- Go 1.27+
- Make
- Podman or Docker with Compose support

### Run locally

Start PostgreSQL, apply migrations, export the required `APP_AUTH_SECRET`, then
start the API:

```bash
cp .env.example .env
export APP_AUTH_SECRET="$(openssl rand -base64 32)"
make compose-up
make migrate-up
go run ./cmd/web
```

`APP_AUTH_SECRET` is required in every environment, including development;
startup fails with a validation error when it is missing or empty. It must be at
least 32 bytes outside development. Follow `.env.example` for the other settings
and set each variable in the process environment.

Health check:

```bash
curl http://localhost:8080/healthz
```

Response:

```json
{"status":"ok"}
```

### Authentication

The API uses `HttpOnly` cookies. Log in, then call protected routes with the
cookie jar:

```bash
curl -c cookies.txt -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'

curl -b cookies.txt http://localhost:8080/users

curl -b cookies.txt -X POST http://localhost:8080/auth/refresh
curl -b cookies.txt -X POST http://localhost:8080/auth/logout
```

The access token lasts 15 minutes and the refresh token lasts 7 days. Refresh
rotates the token pair and revokes the presented session; logout revokes the
presented refresh session. Replaying a rotated token revokes every refresh
session for that user. Password changes, admin password resets, and deactivating
an account also revoke all of that user's refresh sessions. Expired sessions are
deleted opportunistically during login and refresh.

Cookies are `HttpOnly` and `SameSite=Lax`. `Secure` is `true` only when
`APP_ENV=production`; staging and other environments over HTTPS still receive
non-`Secure` cookies, so do not broaden that rule without review. Always set
`APP_AUTH_SECRET`; startup rejects a missing or empty secret in every
environment and a secret shorter than 32 bytes outside development.

### Seed an admin

`cmd/seeder` creates the first admin account from environment variables. It
reads the same `POSTGRES_*` settings as the API, normalizes the email, validates
the password against the registration policy (8–72 bytes), hashes it with
bcrypt, and inserts the user and profile in one transaction:

```bash
export SEEDER_ADMIN_EMAIL=admin@example.com
export SEEDER_ADMIN_PASSWORD='change-me-now'
make seed
```

Both `SEEDER_ADMIN_EMAIL` and `SEEDER_ADMIN_PASSWORD` are required; a missing,
empty, or invalid value fails before the database is touched. The seeder loads credentials from `.env` with Viper; `SEEDER_ADMIN_EMAIL`
and `SEEDER_ADMIN_PASSWORD` process environment variables are ignored. The seeded profile
uses the default full name `System Administrator` and leaves the optional
profile fields empty. The admin public ID uses the usual `YTP-` plus six digits
format. Re-running with an email that already exists fails with "already
registered" and writes nothing. The success log prints only the email and public
ID; the password and hash are never logged.

Rotate the seeded admin password after the first provisioning: log in and use
`PUT /users/{publicID}/password`, or have another admin reset it. Do not keep the
initial `SEEDER_ADMIN_PASSWORD` value in service; remove it from the environment
once the account is in use.

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
| `APP_ENV` | `development` | Application environment; exact `production` enables secure cookies |
| `APP_VERSION` | `dev` | Application version reported at startup |
| `APP_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout for `SIGINT`/`SIGTERM` |
| `APP_AUTH_SECRET` | — (required) | JWT signing secret; no default. At least 32 bytes outside development |
| `APP_AUTH_ACCESS_TOKEN_TTL` | `15m` | Access token lifetime |
| `APP_AUTH_REFRESH_TOKEN_TTL` | `168h` | Refresh token lifetime |
| `APP_CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173` | Credentialed CORS origins |
| `APP_RATE_LIMIT_LOGIN_PER_MINUTE` | `5` | Login requests per minute per client IP |
| `APP_RATE_LIMIT_REFRESH_PER_MINUTE` | `10` | Refresh requests per minute per client IP |
| `APP_RATE_LIMIT_GENERAL_PER_MINUTE` | `60` | General API requests per minute per client IP |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `youthpreneur` | PostgreSQL database |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `SEEDER_ADMIN_EMAIL` | — | Seed admin email; required by `cmd/seeder` |
| `SEEDER_ADMIN_PASSWORD` | — | Seed admin password (8–72 bytes); required by `cmd/seeder` |

`.env` is local-only and ignored by Git. Use `.env.example` as the starting
template; the process environment is what `Viper` reads. `APP_ENV` is the only
supported name for the application environment. The earlier `APP_ENVIRONMENT`
name is not read, so deployments that used it must rename it to `APP_ENV` before
upgrading.

## Operational Constraints

- **Rate limiting is per client IP and in-memory.** It requires direct
  single-process exposure. Behind a reverse proxy every request can share the
  proxy's `RemoteAddr`, turning the login limit into a global limit, and limits
  multiply across replicas. Before a proxied or multi-replica deployment, add
  trusted-proxy client-IP extraction and shared rate-limit storage.
- **Cookies are `Secure` only when `APP_ENV` is exactly `production`.** Staging
  and other HTTPS environments still receive non-`Secure` cookies. Do not
  broaden the rule without a deliberate review.
- **Password change is self-service.** `PUT /users/{publicID}/password` requires
  the current password, so an admin cannot use it to set another user's
  password. Admins use `POST /users/{publicID}/password/reset`.
- **Session revocation.** Refresh tokens rotate on use; a replayed revoked
  token revokes every refresh session for its user. Password change, admin
  password reset, and account deactivation revoke all of a user's refresh
  sessions. Expired sessions are deleted opportunistically during login and
  refresh; no scheduled cleanup job is required.
- **Access tokens are stateless.** Revoking refresh sessions does not invalidate
  an already issued access token; it remains valid for up to its 15-minute
  lifetime. Authorization uses the current database role, so demotions take
  effect on the next request.
- **`GET /users` lists member users only.** Admin accounts are excluded before
  the cursor and limit are applied, so pagination covers members. Use
  `GET /users/{publicID}` to read a specific admin.
- **Enterprise ownership is one-to-many and server-derived.** Any active member
  can create many enterprises, but `enterprises.user_id` always comes from the
  authenticated identity, never from the request. Members list and read only
  their own enterprises; admins list and read all. Owners may update only
  `name`, `business_sector`, `initial_turnover`, and `current_turnover`; admins
  may also update `district`, `status`, and the assessment fields. List filters
  cover `district`, `status`, `business_sector`, `legal_status`,
  `business_digitization`, `intervention_needs`, `training_status`,
  `mentoring_status`, `capital_access`, and `partnership`; name, turnover
  values, IDs, and timestamps are not filterable. Deletes are soft and every
  mutation writes an `enterprise_audit_events` row in the same transaction.
  Updates lock the enterprise row with `SELECT ... FOR UPDATE`, apply only the
  requested fields, and return the committed row without a post-commit re-read.
  Migration `000004` adds database `CHECK` constraints that keep turnover
  non-negative and below `10000000000000`.
- **Training catalog and enrollment.** Catalog reads are public; catalog writes
  are admin-only. Enrollment mutations require authentication and are scoped to
  the current user for members; admins can view all and per-catalog history.
  `user_id` is always server-derived. Capacity is enforced in one transaction
  that locks the catalog row with `SELECT ... FOR UPDATE`, so `training_slots`
  is never exceeded. Cancellation is a soft delete that preserves history, and
  the partial unique active-enrollment index allows re-enrollment afterward.
  Catalog list filters are `category`, `training_status`, `training_date`, and
  `training_period`; cursor pagination uses internal IDs with `limit + 1`.
  Migration `000005` adds the tables, the positive `training_slots` check, the
  active-enrollment unique index, and the catalog/enrollment indexes.

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
internal/delivery/http/handler/  HTTP handlers
internal/delivery/http/route/    route registration and permissions
internal/delivery/http/middleware/ authentication, authorization, CORS, rate limiting
internal/entity/                 domain entities
internal/model/                  HTTP request and response models
internal/repository/             repository interfaces
internal/repository/persistence/ PostgreSQL repository implementations
internal/token/                  access token signing and refresh token generation
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
