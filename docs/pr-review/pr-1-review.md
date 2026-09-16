# PR #1 Review: feat: add user service and database migrations

- PR: https://github.com/tnnz20/youthpreneur-be/pull/1
- Head: `feat/user` (b9f08b3) vs base `master`
- Commits: `18f5b55` (graceful shutdown), `b9f08b3` (user service + migrations)
- Scope: PostgreSQL schema + embedded migration CLI, user/profile entities, repository, use case, HTTP handlers with cursor pagination, bcrypt password handling, graceful shutdown wiring, tests, docs.

## Verification Results (run locally on `feat/user`)

- `gofmt -l .` — no output (clean)
- `go vet ./...` — clean
- `go test ./...` — all packages ok (config, repository, usecase, handler, route cached/ok)
- Repo-claimed "PostgreSQL migration cycle verified" not independently re-run (no local DB provided).

## Summary

Well-structured, standards-library-first implementation. Clean layering (entity / model / repository / usecase / handler), embedded golang-migrate migrations with a small CLI, standard-lib `http.ServeMux` routing, correct keyset (cursor) pagination with limit+1 look-ahead, bcrypt for passwords, transaction-bounded create/soft-delete, and a real integration test (opt-in via `TEST_POSTGRES_DSN`). `cmd/migrate/main.go` handles `ErrNoChange`, `ErrNilVersion`, and dirty-state `force` correctly; `migrationDSN` scheme rewrite to `pgx5` matches the golang-migrate `database/pgx/v5` driver registration. Graceful shutdown path in `cmd/web/main.go` (signal context + `srv.Shutdown` with timeout + `db.Close` defer) is correct.

The main risk class: the PR ships destructive/privileged endpoints with no authentication anywhere, and its own docs acknowledge this — but two of them (password reset, role assignment) are unsafe by default, not just "unwired".

## Findings (severity-ordered)

### Critical

1. **Unauthenticated password reset = account takeover** — internal/delivery/http/route/route.go:31, internal/delivery/http/handler/user.go:192-204, internal/usecase/user.go:229-244.
   `POST /users/{publicID}/password/reset` sets a new password with no credential of any kind. Today, even without auth middleware, this endpoint lets anyone on the network take over any account: public IDs are only 10^6 values (`YTP-DDDDDD`, internal/usecase/user.go:21-23), fully enumeratable. "Auth not included yet" is a fair scoping decision, but shipping an endpoint that resets `users.password` for a known public_id is a hazard even in development (any LAN client can hijack any account now) and a footgun at deploy time. Suggestion: gate `ResetPassword` behind an admin-only auth scope at minimum, or drop the route until auth exists. Same argument with weaker force for `UpdateProfile`/`UpdateStatus`/`Delete`.

### Important

2. **Client-controlled `role` allows self-assignment to `admin`** — internal/delivery/http/handler/user.go:43-44 (`Role: request.Role`), internal/usecase/user.go:281-284 (`RoleAdmin` accepted from raw request). Any unauthenticated visitor can register as `admin` and store that value in the DB. Since admin role presumably grants authority once auth/middleware lands, poisoned seed data becomes a persistent privilege backdoor. Suggestion: ignore `request.Role`, or default to `member` server-side (hard-set unless a trusted path exists).

3. **Unbounded request bodies — DoS vector** — internal/delivery/http/handler/user.go:206-213. `decode` reads `r.Body` with no `http.MaxBytesReader` and no (optional) `DisallowUnknownFields`. An attacker can stream arbitrarily large bodies into memory. Cheap fix: wrap `r.Body = http.MaxBytesReader(w, r.Body, 1<<20)` in `decode`.

4. **`ErrInvalidInput` chain leaks internal sentinel text to clients** — internal/delivery/http/handler/user.go:231 (`h.writeError(w, 400, err.Error())`). Clients receive `"usecase: invalid input: invalid email"` including the internal sentinel prefix. Not exploitable, but sloppy API surface and inhibits error-message changes. Suggestion: map to short stable messages (or `err := fmt.Errorf("%w", errors.Unwrap(...))` / dedicated message strings).

### Minor

5. **Public ID collision ceiling is low and unbounded growth is unhandled** — internal/usecase/user.go:20-26, 343-350. 10^6 space with 5 `rand.Int` retries is fine while user count is ≪ 10^5 (birthday bound), but this format cannot survive growth and the retry loop silently degrades. At minimum document the ceiling (e.g. `ponytail:`-style comment) or widen the ID (8+ digits/alphanumeric) before scale-out.

6. **bcrypt 72-byte input truncation accepted silently** — internal/usecase/user.go:292-298. `validatePassword` checks only minimum length; bcrypt truncates at 72 bytes. Functionally fine, cosmetic. Optionally set a max (e.g. 72) with a clear message.

7. **Create flow registration order lets enum mismatch slip through on rollback** — internal/db migration 000001 registers `user_role` as a Postgres enum while role validation also lives app-side (internal/usecase/user.go:282-284). Duplicate defense is good; just be aware a future direct SQL insert can fail with `invalid_enum_value` and that a new enum member requires an app-side update too.

8. **Application `mysql`-style `FROM users` delete path in second UPDATE of same tx can be side-ordered** — internal/repository/user_postgres.go:168-178 already correctly derives the profile row from `users` by `public_id`, so no issue found there; noted as verified rather than a defect.

9. **Graceful-shutdown error branch skips deferred closes** — cmd/web/main.go:54. `os.Exit(1)` on `ListenAndServe` failure bypasses `defer db.Close()`. Harmless here (process death closes sockets) but a `return` with error status would honor defers. Accepted as-is.

10. **`birth_date` accepts future dates** — internal/delivery/http/handler/user.go:257-268. `time.Parse("2006-01-02", ...)` doesn't reject dates after today. Validation gap only.

11. **`ChangePassword` is a two-step read-then-write without a transaction/lock** — internal/usecase/user.go:199-227 + internal/repository/user_postgres.go:271-290. A racing soft delete or second password change can interleave; the outcome is "one update silently lands after another" — schema does not lose data, and `requireAffected` guards deletion, so exposure is minimal. Note for later: single `UPDATE ... WHERE password = current_hash` statement would make it atomic.

12. **`migrationDSN` silently falls back on parse failure** — cmd/migrate/main.go:119-128. If `url.Parse` ever failed, the raw `postgres://` DSN goes to a driver registered only as `pgx5`, producing a confusing "not registered" error. In practice `PostgresConfig.DSN()` is always well-formed, so error is unreachable; worth one `return fmt.Errorf` instead of fallback.

## Suggestions

- Reconsider whether `POST /users/{publicID}/password/reset` and client-facing `role` belong in this PR. Removing both is ~10 lines of deletion and eliminates findings 1 and 2; auth-awareness can include these in a minimum-time state without leaving a live footgun.
- Add `usecase.ErrBadRequest` paths to the handler's `writeUsecaseError` variant rather than echoing `err.Error()`.
- Consider a `context.Context` in profile `UpdateProfile` scan for "user exists but has no profile row" — currently profile rows are always created in `CreateUser`, so documented invariant. Fine.
- Skipped tests for `parseBirthDate`/`UpdateStatus` handler paths exist in part; the residual untested handler paths (UpdateProfile, status, password endpoints) would benefit from table tests once finalized. Both repo and handler error tests are otherwise well covered.

## Positive Notes

- Correct cursor pagination: `limit+1` look-ahead, `NextCursor = users[limit-1].ID`, omitted `next_cursor` on last page (internal/usecase/user.go:246-273, verified by TestFindUsers*).
- Transaction boundaries correct: create (user + profile) and soft delete (user + profile) share one tx and one timestamp; rollback defers are deliberate (internal/repository/user_postgres.go:46-99, 147-185).
- ErrTxDone-aware rollback in delete/update paths, and `requireAffected` translating zero-affected updates to `ErrUserNotFound`.
- Password never serialized to API responses (internal/model/user.go:58) and never returned by handlers (`toUserResponse` omits it).
- Migration CLI treats `ErrNoChange` as success, `ErrNilVersion` as "no migrations applied", and exposes `force` for dirty recovery (cmd/migrate/main.go:62-115), with correct `pgx5` scheme rewrite.
- Enum + unique + soft delete + epoch timestamp design is coherent and well-documented in `README.md` including the no-auth limitation, and traceability matches `CHANGELOG.md`.
- Solid seed test surface: fake-repo-driven usecase tests, SQL-specialist mapping tests, and opt-in real-DB integration test.

## Final Conclusion / Recommendation

**Request Changes (blocking)**
- Critical #1 (password reset exposure) and Important #2 (role self-assignment) must be addressed before merge — either by deleting the endpoints/inputs or by gating them now.
- Important #3 and #4 are small, cheap fixes that fit the PR surface.

Schema, migration CLI, pagination, password handling, transaction boundaries, and shutdown behavior are all correct for what ships. Once 1/2/3/4 are resolved, this is a clean, well-tested contribution ready for merge.
