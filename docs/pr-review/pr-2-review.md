# PR #2 Review: `feat/auth`

## Scope

Reviewed authentication, refresh-token persistence, authorization middleware, CORS, rate limiting, migrations, tests, documentation, duplicate code, and unused code against `origin/master`.

## Verification

- `go build ./...` passed.
- `go vet ./...` passed.
- `go test -count=1 ./...` passed.
- Context propagation is correct: handler → `r.Context()` → usecase → repository.
- Route permissions match `api/api-contract.md`.
- Refresh rotation is atomic and concurrent replay loses the race.
- Raw refresh tokens are not stored; only SHA-256 hashes are persisted.
- Login timing for unknown email is mitigated with a dummy bcrypt hash.

## High Findings

### H1. Default auth secret passes production validation

Location: `internal/config/config.go:76-100`

`defaultAuthSecret = "dev-only-insecure-secret-change-me"` is longer than the 32-byte minimum, so production startup can succeed without `APP_AUTH_SECRET`. The service then signs JWTs with a source-controlled secret.

**Suggestion:** Reject the exact default secret outside development, in addition to minimum-length validation. Add a regression test for missing/default production secret.

### H2. Password changes do not revoke refresh sessions

Locations: `internal/usecase/user_usecase.go:264-319`, `internal/repository/auth_repository.go:37`, `internal/repository/persistence/auth_postgres.go:137`

`RevokeUserRefreshSessions` exists but has no production caller. A password change or admin reset leaves stolen refresh sessions valid for up to 7 days.

**Suggestion:** Revoke all user refresh sessions after successful `ChangePassword` and `ResetPassword`. Consider revoking sessions when an account is disabled.

### H3. Rate limiting uses proxy address as client identity

Location: `internal/delivery/http/middleware/ratelimit.go:96-106`

Behind a reverse proxy, all requests may share one `RemoteAddr`. Login limits can become a global 5 requests/minute limit. In-memory limits also multiply across replicas.

**Suggestion:** At minimum document that current limiter requires direct single-process exposure. Before proxy deployment, add trusted-proxy IP extraction and shared rate-limit storage.

## Medium Findings

### M1. Replay does not revoke refresh-token family

Location: `internal/usecase/auth_usecase.go:139-161`

A replayed rotated token is rejected, but replacement sessions remain valid.

**Suggestion:** Revoke all refresh sessions for the user when replay of a revoked token is detected.

### M2. Role middleware trusts stale JWT role claims

Locations: `internal/delivery/http/middleware/auth.go:79-133`

Authentication loads the current DB user but role checks use JWT claims. A demoted admin can retain admin access until the access token expires.

**Suggestion:** Compare token role with current DB role or use current DB role for authorization.

### M3. Refresh-session table has no cleanup

Location: `db/migrations/000002_create_refresh_sessions.up.sql`

Expired and revoked sessions accumulate indefinitely. The expiry index is not used by cleanup.

**Suggestion:** Add opportunistic cleanup or document and provision scheduled cleanup.

### M4. Inactive-account login timing differs

Location: `internal/usecase/auth_usecase.go:107-115`

Inactive accounts return before bcrypt verification, allowing timing-based distinction from unknown users and wrong passwords.

**Suggestion:** Compare password first, then check `IsActive`.

## Low Findings

- Duplicate error writers: `internal/delivery/http/middleware/auth.go:121-133` and `internal/delivery/http/handler/response.go:30-34`. Consolidate shared response writing.
- Contract hardcodes rate limits even though `APP_RATE_LIMIT_*` values are configurable.
- `APP_ENVIRONMENT` to `APP_ENV` is a breaking environment rename; document migration clearly or support a temporary alias.
- `staging` over HTTPS receives non-`Secure` cookies because only exact `production` enables `Secure`.
- Disallowed CORS responses bypass general rate limiting because CORS wraps the limiter.
- Admin path through `RequireSelf` for password change is technically allowed but practically requires knowing target user's current password; admin reset is the intended path.
- JWT signing-method type assertion duplicates `jwt.WithValidMethods` checking.
- `RateLimiter.Allow` is exported but appears test-only.

## Duplicate and Unused Code

### Duplicate

- `writeJSONError` in middleware duplicates `writeError` in handler response helpers.
- Test stubs with the same name exist in separate packages; this is acceptable and not production duplication.
- User handler response/decode helpers were consolidated correctly.

### Unused or dead

- `RevokeUserRefreshSessions` is currently unused in production. It should be wired into password changes, password resets, account disabling, and replay response, or removed if those policies are intentionally deferred.
- `RateLimiter.Allow` is exported without an apparent production caller; make it private unless external use is planned.

## Missing Tests

- Password change/reset revokes all refresh sessions.
- Refresh replay revokes session family.
- Production default/missing auth secret is rejected.
- Current DB role overrides stale JWT role.
- Inactive-account password comparison occurs before rejection.
- Rate limiter behavior under `-race` and concurrent requests.
- Expired-token cookie max-age handling.

## Conclusion

Architecture is sound: cookie-based JWT access tokens, opaque hashed refresh tokens, atomic rotation, parameterized SQL, context propagation, route authorization, CORS, rate limiting, and broad unit coverage are present.

## Merge Recommendation

**Request changes before merge.** Fix H1 and H2. Document H3's direct single-process deployment constraint before using this behind a proxy. Medium findings should follow before production deployment, especially stale role claims and refresh-session cleanup.

## Resolution Status (2026-09-17)

- H1: Fixed. `APP_AUTH_SECRET` has no default; missing/empty is rejected in every
  environment and the 32-byte minimum remains outside development.
- H2: Fixed. Password change, admin reset, and deactivation revoke the user's
  refresh sessions.
- H3: Documented. `README.md` and `api/api-contract.md` state the direct
  single-process requirement and the trusted-proxy/shared-storage prerequisites.
- M1: Fixed. Replay of a revoked token revokes the user's whole session family.
- M2: Fixed. Authorization uses the current database role, not the JWT claim.
- M3: Fixed. Expired sessions are deleted opportunistically during login and
  refresh.
- M4: Fixed. Login always performs the bcrypt comparison before the active
  check.
- Low findings: duplicate error writers consolidated, rate limits documented as
  overridable, `APP_ENV` rename documented, exact `Secure` rule documented,
  CORS ordering covered by rate limiting, password change/reset split
  documented, redundant JWT assertion removed, `RateLimiter.Allow` made private.
- Deferred: trusted-proxy client-IP extraction and shared rate-limit storage,
  and broadening cookie `Secure` beyond exact `APP_ENV=production`.
