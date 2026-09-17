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
- Cookie-based JWT authentication with `POST /auth/login`, `POST /auth/refresh`,
  and `POST /auth/logout`.
- `refresh_sessions` migration and repository storing SHA-256 refresh token
  hashes with atomic rotation, revoke, and revoke-all operations.
- Access token signing service plus opaque refresh token generation and hashing.
- Authentication, role, and ownership middleware that validates access cookies
  and rejects inactive or deleted users.
- Credentialed CORS middleware and per-client-IP fixed-window rate limiting with
  `429` and `Retry-After`.
- Auth, CORS, rate-limit, and `APP_ENV`-derived secure cookie configuration.
- `enterprises` and `enterprise_audit_events` migrations with
  `business_sector_enum`, `enterprise_status_enum`, `legal_status_enum`,
  `business_digitization_enum`, `intervention_needs_enum`,
  `process_status_enum`, and `general_status_enum` types, `DECIMAL(15,2)`
  turnover columns, nullable `name` and assessment fields, soft delete, and
  owner/filter/cursor indexes.
- Enterprise repository, use case, and HTTP handlers for create, list, get,
  partial update, and soft delete, scoped one-to-many by owner.
- Enterprise routes under `/enterprises`, authenticated for members and admins
  with owner-scoped reads for members and unrestricted reads for admins.
- Transactional enterprise audit events recording the actor, action, and
  changed fields JSONB for every create, update, and delete.
- `training_catalog` and `training_enrollments` migration `000005` with
  reusable `process_status_enum`, nullable catalog fields, a positive
  `training_slots` `CHECK` constraint, a partial unique active-enrollment index,
  and catalog/enrollment filter and cursor indexes.
- Training catalog repository, use case, and HTTP handlers for public list and
  get plus admin create, partial update, status update, and soft delete.
- Training enrollment repository, use case, and HTTP handlers for authenticated
  self-enrollment, cancellation, and own history, plus admin all-history and
  per-catalog history.
- Transactional capacity safety that locks the catalog row with
  `SELECT ... FOR UPDATE`, rejects duplicates, closed, and full catalogs with
  `409`, and preserves cancellation history so users can re-enroll.

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
- All non-public routes now require authentication; admin routes require the
  `admin` role, and profile and password updates require ownership or admin.
- Password reset is restricted to authenticated admins.
- `config.Bootstrap` validates `APP_AUTH_SECRET` outside development and returns
  the middleware-wrapped `http.Handler`.

- Centralized dependency wiring in `config.Bootstrap`, which now opens and
  verifies the PostgreSQL connection.
- Renamed health use case identifiers to follow Go naming conventions.
- `config.Load` now exposes `POSTGRES_SSLMODE` and builds a PostgreSQL DSN.
- Moved embedded SQL migrations from `migrations/` to `db/migrations/`.
- User soft delete now updates `users` and `user_profiles` in one transaction
  with the same `deleted_at` and `updated_at` timestamp.

### Fixed

- `APP_AUTH_SECRET` no longer has a usable default: startup rejects a missing or
  empty secret in every environment and keeps the 32-byte minimum outside
  development.
- Password change, admin password reset, and account deactivation now revoke the
  user's refresh sessions; resetting a password no longer leaves stolen refresh
  sessions valid.
- Replaying a rotated refresh token now revokes the user's entire refresh
  session family instead of only rejecting the replayed token.
- Expired refresh sessions are deleted opportunistically during login and
  refresh, so the table no longer grows without bound.
- Role and ownership checks now use the current database role loaded by
  `Authenticate` instead of the JWT role claim, so stale admin claims cannot
  retain access after demotion.
- Login verifies the password before checking `IsActive`, so inactive accounts
  cannot be distinguished from unknown emails or wrong passwords by timing.
- Middleware error responses now share the handler `WriteError` helper instead
  of a duplicate writer; the exported `RateLimiter.Allow` was made private.
- General rate limiting now wraps CORS so disallowed-origin requests are also
  counted, and expired cookie expiries are clamped to `MaxAge=0`.
- Removed the redundant JWT signing-method type assertion that duplicated
  `jwt.WithValidMethods`.
- Documented the single-process rate-limiter limitation, the exact `APP_ENV`
  cookie `Secure` rule, the `APP_ENVIRONMENT` to `APP_ENV` rename, session
  revocation and cleanup behavior, and the password change/reset split.
- Removed the duplicated `writeJSONError` middleware helper and the stale
  "unsafe" note on admin password reset.
