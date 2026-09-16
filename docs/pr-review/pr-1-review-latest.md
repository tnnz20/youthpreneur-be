# PR #1 Review (Latest): feat: add user service and database migrations

- PR: https://github.com/tnnz20/youthpreneur-be/pull/1
- Head: `feat/user` @ `822f3fe` ("docs: add API contract and organize packages") vs base `master`
- Commits since prior review (`docs/pr-review/pr-1-review.md`, head `b9f08b3`):
  - `3d9a545` docs: mark password reset unsafe before auth
  - `2b316f1` fix: address pull request review findings
  - `822f3fe` docs: add API contract and organize packages
- Scope of this review: full diff vs `master`, all 5 commits, prior findings reconciliation, duplicate/unused/dead code, stale references, package organization, API contract accuracy, tests, migration behavior, security, Go quality.

## Verification Results (run locally on latest `feat/user`)

- `gofmt -l .` — clean (no output)
- `go vet ./...` — clean
- `go test ./...` — all packages ok (config, usecase, handler, route, persistence, cmd/migrate)
- `go mod tidy -diff` — no changes (dependency tree is tidy; no unused deps)
- `deadcode` (golang.org/x/tools/cmd/deadcode) — no unreachable functions
- PostgreSQL migration cycle not independently re-run (no local DB provided; integration test remains opt-in via `TEST_POSTGRES_DSN`, wired correctly at internal/repository/persistence/user_test.go:60-64)

## Status of Prior Findings

| # | Prior finding | Status |
| --- | --- | --- |
| 1 (Critical) | Unauthenticated password reset = account takeover | **UNRESOLVED** — see Remaining Findings R1 |
| 2 (Important) | Client-controlled `role` allows self-assignment to `admin` | **FIXED** — `Role` is accepted for wire compat but ignored; `CreateUser` hard-sets `RoleMember` (internal/usecase/user_usecase.go:76-80, 133-136). Documented in api-contract.md:57-59, 122-124 |
| 3 (Important) | Unbounded request bodies (DoS) | **FIXED** — `decode` now wraps `r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)` with `maxBodyBytes = 1 MiB` (internal/delivery/http/handler/user_handler.go:19-20, 211-213) |
| 4 (Important) | Internal sentinel text leaked to clients | **FIXED** — new `usecase.BadRequestError` carries a client-safe `Message`, unwraps to `ErrBadRequest`; `writeUsecaseError` surfaces only the message with a stable `"invalid request"` fallback (internal/usecase/user_usecase.go:37-70, internal/delivery/http/handler/user_handler.go:234-255) |
| 5 (Minor) | Low public ID collision ceiling, silent degradation | **FIXED** — ceiling documented on `publicIDMax`, bounded retries now fail with `ErrPublicIDGeneration` instead of degrading silently (internal/usecase/user_usecase.go:22-26, 148-182) |
| 6 (Minor) | bcrypt 72-byte silent truncation | **FIXED** — `maxPasswordLength = 72` enforced with explicit message (internal/usecase/user_usecase.go:27-30, 331-340) |
| 7 (Minor) | Dual role validation (PG enum + app) | **Unchanged / accepted** — still duplication of defense; behavior identical to prior review. Informational only |
| 8 (Minor) | Side-ordered delete path | Was verified correct, not a defect — N/A |
| 9 (Minor) | `os.Exit(1)` skipped deferred closes on shutdown error | **FIXED** — `main` now calls `run()` and every exit path returns through deferred `db.Close()` (cmd/web/main.go:16-40) |
| 10 (Minor) | `birth_date` accepts future dates | **FIXED** — `parseBirthDate` rejects dates after now (internal/delivery/http/handler/user_handler.go:270-284). Applied to both Create and UpdateProfile |
| 11 (Minor) | `ChangePassword` read-then-write race | **FIXED** — repository update now predicates on stored hash: `WHERE public_id = $1 AND password = $2 AND deleted_at IS NULL`; zero rows maps to `ErrInvalidCredentials` (internal/repository/persistence/user_postgres.go:293-314, internal/usecase/user_usecase.go:256-268) |
| 12 (Minor) | `migrationDSN` silently fell back on parse failure | **FIXED** — parse failure returns an error instead of passing the unregistered `postgres://` scheme through (cmd/migrate/main.go:122-135) |

Summary: **11 of 12 prior findings addressed** (one was non-defect), **1 remains unresolved**.

## Summary

The fix commit (`2b316f1`) and the package-organization commit (`822f3fe`) address every actionable finding from the prior review except the password-reset exposure. The error-surface redesign (`BadRequestError` + `writeUsecaseError`) is a clean pattern: sentinel `ErrBadRequest` stays matchable via `errors.Is`, clients get stable short messages, internal prefixes never leak. Lifecycle handling in `cmd/web/main.go` is now idiomatic (`run() error` + defers). Package renaming (`health_handler.go`, `user_usecase.go`, `user_repository.go`, `user_postgres.go`) removes ambiguity from the flat domain directories; there are no stale imports — everything compiles and grep finds the old names only inside historical plan documents (see R4). The new `api-contract.md` is accurate against the code (verified in detail below). Tests are extensive and all pass: fake-repo usecase tests (511 lines), handler round-trip tests (370), SQL mapping tests (186), config/logger tests, migrate CLI usage test, opt-in real-DB integration test.

## Remaining Findings (severity-ordered)

### R1 — Critical (prior #1 unresolved)

**Unauthenticated `POST /users/{publicID}/password/reset` is still live** — internal/delivery/http/route/route.go:31, internal/delivery/http/handler/user_handler.go:194-209, internal/usecase/user_usecase.go:271-286.

The commit `3d9a545` mitigates by documentation only (api-contract.md "Safety limitations", README). The route, handler, and usecase code are unchanged: anyone who can reach the server can set an arbitrary new password for any known public ID. The public ID space is still 10^6 values and clearly documented as "not secret", making enumeration trivial. All docs now consistently say "do not expose outside a trusted development network" — this is a policy guard, not a control. Once this service reaches any shared network or deployment, the endpoint enables full account takeover.

Decision needed: either (a) delete the route/handler/usecase/model/impl (~30 lines of deletion) and reintroduce with auth, or (b) accept and gate with a documented interim control in this PR. Documentation alone does not satisfy the prior review's blocking status, and this reviewer concurs it should not: the fix-level git history can carry the removal; "auth lands soon" has prevented nothing so far.

### R2 — Minor

**Mutating public endpoints without auth** (Delete, UpdateProfile, UpdateStatus) — inherent to the declared "auth not included" scope and consistently documented (api-contract.md, README). Acceptable as scoped; the same LAN-enumeration argument as R1 applies with lower force. No change required in this PR.

### R3 — Minor (new)

**Duplicated shutdown-timeout default** — config.go:61 (`v.SetDefault("app.shutdown_timeout", "10s")`) and config.go:52 (`defaultShutdownTimeout = 10 * time.Second`, used only as a fallback for parsed zero). Two sources of truth for one value. Collapse to one (drop the constant or the string default). Trivial.

### R4 — Minor (new)

**Stale paths in historical plan docs** — docs/superpowers/plans/2026-09-17-user-service-migrations.md references `internal/usecase/user.go`, `internal/delivery/http/handler/user.go`, `internal/repository/user_postgres.go` under `db/`-style paths that no longer match the renamed files. Plans are point-in-time records; no code impact. Ignore or add a header note; do not rewrite history.

## Duplicate / Unused / Dead Code

- `deadcode` analysis: no unreachable functions anywhere in the module.
- `go mod tidy -diff`: no unused requirements.
- Handler-side profile field mapping (user_handler.go:50-58 and 137-145) duplicates the `entity.Profile{...}` literal across Create and UpdateProfile. ~8 lines; extracting a helper for two call sites is not worth it. Accept.
- Role validation exists twice by design (PG enum in migration 000001 + app-side `validateGender`-style checks). Deliberate defense-in-depth; documented in prior review. Accept.
- No stale references in compilable code: file renames (`health.go` → `health_handler.go` etc.) leave no dangling imports or tests.
- `ResetPassword`, if R1 is resolved by deletion, should be deleted across all five layers (route, handler, usecase method, model request, repo `UpdatePassword`) — a partial removal would create genuinely dead code.

## API Contract Accuracy (api-contract.md vs code)

Verified end-to-end:

- Routes, methods, paths (api-contract.md:44-53) match route.go registrations exactly, including `/healthz`.
- Status codes: 201 create, 200 others, 204 delete/passwords, 400/401/404/409/500 mapping matches `writeUsecaseError` and handler branches.
- "Bodies capped at 1 MiB" matches `maxBodyBytes`.
- Password rules "8 to 72 bytes" match `minPasswordLength`/`maxPasswordLength`.
- "role accepted but ignored, always member" matches usecase behavior.
- Pagination: "clamped to 1-100; default 20" matches `clampLimit`; "ordered by id ascending, no offset" matches `findUsersQuery`; non-positive limit → 400 matches `parseLimit`; omitted `next_cursor` on last page matches handler List (user_handler.go:96-98).
- Response shapes match `internal/model` tags; password hash never serialized (verified in both model and `toUserResponse`).
- Cursor as string of last seen `users.id` matches `NextCursor` construction.
- Contract does not mention `Cursor` type coercion subtleties beyond 400 behavior — fine.

Contract is accurate. No corrections needed.

## Migration Behavior

- `migrationDSN` now fails fast on unparseable DSN; `pgx5` scheme rewrite correct for the registered `database/pgx/v5` driver.
- `ErrNoChange` → success, `ErrNilVersion` → "no migrations applied", `force VERSION` for dirty recovery, `version` reports dirty state. All paths carry tests (cmd/migrate/main_test.go).
- Schema unchanged from prior review: PG enum `user_role`, unique `public_id`/`email`, soft delete columns, district/gender indexes supporting the list filters. Down migration drops both tables and the enum.
- Not independently re-run against PostgreSQL in this session (no DB available); engine logic verified by code review and unit tests.

## Security Review

- Password hashing: bcrypt `DefaultCost`, length-bounded both directions. No plaintext or hash ever in responses or logs.
- Password reset: live hazard (R1), only documentation guard.
- Request bodies: bounded at 1 MiB. Unknown JSON fields are silently ignored (not rejected) — acceptable, and consistent with wire-compat goals.
- Error handling: no internal error text or stack reaches clients; 500s log server-side.
- Injection: all queries parameterized; no string-built SQL except static column list `userColumns` (trusted constants).
- Enumeration: public IDs are documented lookup handles; `Get DELETE ChangePassword` decide with 404 vs 401 — current behavior (404 for unknown user) is standard but leaks existence to unauthenticated callers; consistent with the no-auth scope.
- Secrets: `.env.example` contains only placeholder-style defaults; no secrets committed.

## Go Quality

- `gofmt`/`vet` clean; dependency graph tidy; no dead code.
- Consistent wrapping (`fmt.Errorf(... %w ...)`), sentinel errors at each layer boundary, `errors.Is`/`errors.As` used correctly throughout.
- Idiomatic Go pattern: `run() error` in both binaries; deterministic defer cleanup; signal context.
- Comments explain intent and invariants (bcrypt bounds, pgx scheme rewrite, atomic predicate) without noise.

## Suggestions

1. Resolve R1 by deletion of the reset endpoint across all layers (preferred: smallest safe diff) or by an explicit interim gate; do not merge with documentation as the only control.
2. Collapse the duplicate 10s shutdown default (R3) — one line.
3. Optional: table-driven handler tests for UpdateProfile/UpdateStatus/password endpoints are partially present; the fake use case pattern supports adding the gap paths cheaply. Not blocking.
4. Optional: consider `DisallowUnknownFields` in `decode` once clients stabilize; not needed now.

## Final Conclusion / Recommendation

**Request Changes — one blocking item.**

Every prior finding except the password-reset exposure (prior #1) is fixed correctly, verified by tests, and the fixes are faithful to the review suggestions rather than cosmetic. Package organization, API contract accuracy, test coverage, Go quality, and migration handling all improved and pass all local checks. The single remaining item is the same live unauthenticated password-reset route; documentation was added but the code-level hazard persists. Remove the endpoint or gate it before merge. Everything else is merge-ready.
