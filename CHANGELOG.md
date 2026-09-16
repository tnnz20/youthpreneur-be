# Changelog

## Unreleased

### Added

- Viper environment configuration.
- Configurable `log/slog` logging.
- `GET /healthz` endpoint.
- PostgreSQL 16 Alpine Compose service.
- Dockerfile and Makefile development commands.
- Go documentation comments for exported identifiers.
- `cmd/migrate` CLI with `up`, `down`, `force`, and `version` commands.
- Embedded SQL migrations for `user_role`, `users`, and `user_profiles`.
- User repository, use case, and HTTP handlers for create, list, get, delete,
  profile update, status update, password change, and password reset.
- `YTP-` public IDs with collision retry, bcrypt password hashing, and cursor
  pagination by `users.id`.
- User profiles with `full_name`, `nik`, `birth_date`, `gender`, `district`,
  `phone`, and `address`, backed by `SERIAL` integer primary keys and a
  `users.password` hash column.
- `phone` and `address` profile fields in the user API request and response
  models.

### Changed

- Centralized dependency wiring in `config.Bootstrap`, which now opens and
  verifies the PostgreSQL connection.
- Renamed health use case identifiers to follow Go naming conventions.
- `config.Load` now exposes `POSTGRES_SSLMODE` and builds a PostgreSQL DSN.
- Moved embedded SQL migrations from `migrations/` to `db/migrations/`.
- User soft delete now updates `users` and `user_profiles` in one transaction
  with the same `deleted_at` and `updated_at` timestamp.
