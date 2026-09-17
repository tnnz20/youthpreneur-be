# Authentication and Middleware Design

## Goal

Add cookie-based JWT authentication with PostgreSQL-backed refresh-token rotation, authorization middleware, CORS, and IP rate limiting.

## Decisions

- Access token lifetime: 15 minutes.
- Refresh token lifetime: 7 days.
- Access and refresh tokens use `HttpOnly` cookies.
- Cookie `Secure` is `true` only when `APP_ENV=production`; otherwise `false` for local HTTP.
- Refresh tokens are cryptographically random opaque values. Store only SHA-256 hashes in PostgreSQL.
- Refresh token rotation revokes consumed tokens and creates replacement tokens.
- Logout revokes presented refresh token and clears both cookies.
- JWT signing uses `github.com/golang-jwt/jwt/v5` with an environment-provided secret.
- Login identifies users by normalized email and verifies bcrypt password.

## Routes and Authorization

Public routes: `GET /healthz`, `POST /users`, `POST /auth/login`, `POST /auth/refresh`.

Authenticated routes: `POST /auth/logout`, `PUT /users/{publicID}/profile` for own user, `PUT /users/{publicID}/password` for own user.

Admin routes: `GET /users`, `GET /users/{publicID}`, `DELETE /users/{publicID}`, `PATCH /users/{publicID}/status`, `POST /users/{publicID}/password/reset`, and profile updates for any user.

Authentication middleware validates access JWT, rejects missing, malformed, expired, inactive, or deleted users, and stores typed claims in request context. Role middleware checks claims. Ownership middleware compares authenticated public ID with route public ID, allowing admins through.

## Middleware

CORS reads allowed origins from existing configuration/environment, defaults locally to `http://localhost:3000,http://localhost:5173`, supports credentials, handles preflight, and rejects unconfigured origins. Rate limiting uses standard library state keyed by client IP with endpoint-specific limits: login 5/minute, refresh 10/minute, general API 60/minute. State is bounded by expiration cleanup and returns `429` with `Retry-After`.

## Persistence

Add migration for refresh sessions containing token hash, user ID, expiry, created timestamp, revoked timestamp, and replacement linkage where useful. Repository methods create, consume-and-revoke, revoke, and optionally revoke all sessions. All DB calls receive `r.Context()` through handler, usecase, and repository layers.

## Configuration and Safety

Add auth secret and CORS configuration through Viper. Startup rejects missing/weak auth secret outside development. Cookies use `SameSite=Lax`, explicit paths, and expiration. Passwords and token values never enter logs or responses. Refresh token hashes enable logout/revocation without storing bearer secrets.

## Testing

Add unit tests for token creation/validation, cookie flags, authentication and authorization decisions, rate-limit windows, CORS behavior, and auth handlers. Add repository integration coverage gated by `TEST_POSTGRES_DSN`. Run `go test ./...`, `go vet ./...`, and `git diff --check`.

## Documentation

Update `api/api-contract.md`, `AGENTS.md`, `.env.example`, and README configuration/setup sections to describe routes, cookie behavior, local versus production `APP_ENV`, refresh rotation/revocation, CORS, and rate limits.
