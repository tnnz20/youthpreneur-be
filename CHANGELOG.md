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

- User creation ignores any client-supplied `role` and always stores `member`;
  admin provisioning requires a trusted authenticated path.
- JSON request bodies are limited to 1 MiB, and validation errors return safe
  stable messages instead of the internal `usecase:` sentinel prefix.
- Passwords longer than bcrypt's 72-byte input limit are rejected instead of
  silently truncated.
- Password change uses an atomic update guarded by the current hash; a lost
  update surfaces as invalid credentials.
- Future `birth_date` values are rejected.
- `cmd/web` returns through a cleanup-safe error flow so deferred database close
  runs on every exit path.
- `cmd/migrate` reports DSN parse failures instead of silently falling back to
  the unregistered `postgres://` scheme.
- Documented the six-digit public ID ceiling and that public IDs are not
  authentication credentials.
- Marked password reset as unsafe until authentication middleware restricts it
  to authenticated admins.

- Centralized dependency wiring in `config.Bootstrap`, which now opens and
  verifies the PostgreSQL connection.
- Renamed health use case identifiers to follow Go naming conventions.
- `config.Load` now exposes `POSTGRES_SSLMODE` and builds a PostgreSQL DSN.
- Moved embedded SQL migrations from `migrations/` to `db/migrations/`.
- User soft delete now updates `users` and `user_profiles` in one transaction
  with the same `deleted_at` and `updated_at` timestamp.
