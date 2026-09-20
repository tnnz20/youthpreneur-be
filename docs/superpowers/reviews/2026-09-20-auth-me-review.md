# Code Review: GET /auth/me (`feat/auth-2`)

**Date:** 2026-09-20
**Base:** `master` (`3613d08`)
**State:** uncommitted working-tree changes on `feat/auth-2` (9 files, +245/-38) plus the plan doc `docs/superpowers/plans/2026-09-20-auth-me-implementation.md` (untracked).
**Scope reviewed:** `git diff master` on `internal/model/auth.go`, `internal/delivery/http/handler/auth_handler.go`, `internal/delivery/http/route/route.go`, `internal/config/bootstrap.go`, tests, and `api/api-contract.md`.

## Verification Performed

- `go vet ./...` — clean
- `go test ./... -count=1` — all packages pass
- `git diff --check` — clean (only CRLF autocrlf warnings)
- `gofmt -l .` — none of the changed files are unformatted (pre-existing unformatted files elsewhere are untouched and out of scope)

## Conclusion

Feature is **ready with one minor follow-up suggested**. Implementation is small, matches the plan, follows existing patterns (`IdentityFunc` injection like enterprise/training handlers), adds no new dependency, usecase, or repository method, and is fully aligned with the API contract. No security issues found. Tests pass and cover the important behaviors.

## Findings

### Critical

None.

### Important

None.

### Minor

1. **`Me` error path duplicates the middleware 401 shape by coincidence, not construction** — `auth_handler.go:70` writes `WriteError(..., 401, "unauthorized")` when identity is missing. This is correct and consistent with `middleware/auth.go:72,78,87,91`, but it is dead code in the wired route: `GET /auth/me` sits behind `Authenticate`, which already returns 401 for missing/invalid tokens, so the fallback only triggers for a misconstructed `AuthHandler` (e.g., `nil` identity as in `auth_handler_test.go:54`) or a future route change. Keeping it is cheap defensive coding; acceptable as-is. No change needed.

2. **`TestMeReturnsMinimalIdentityShape` leaks assertion is string-based** — `protected_route_test.go:196` checks raw body strings for `Bandung`/`08123456789`. Fine in practice because the typed assertions above already pin the shape; a full `model.MeResponse` strict decode could not catch extra fields either (Go ignores unknown JSON fields). Current approach is the pragmatic one. No change needed.

3. **Pre-existing pattern note, not introduced here:** `toMeResponse` (auth_handler.go:173) and `toUserResponse` (user_handler.go:282) map the same `entity.User` into two different response models. This is intentional separation (`MeResponse` is minimal, `UserResponse` is full) and matches the plan's constraint to keep them independent. Do not merge them.

### Suggestions (non-blocking)

- **Nil identity defensive default in tests:** `auth_handler_test.go:54` passes `nil` as the `IdentityFunc`. This works only because `Me` is never invoked there. If someone later routes `Me` through that test mux, it would panic. Consider a harmless no-op identity for the auth test router, e.g. `func(context.Context) (entity.User, bool) { return entity.User{}, false }` — three lines, prevents a latent panic. Optional.
- **Contract wording nit:** `api-contract.md` section 5 says errors are `401` and `500`. Accurate. The route has no other failure mode, so nothing is missing. No action.
- **Doc checklist quality:** plan doc Tasks 1–5 checkboxes all claim done and match reality; the verification claims (`gofmt`, `go test`, `go vet`, `git diff --check`) re-verified clean during this review.

## Checklist Assessment

| Area | Result |
| --- | --- |
| Duplicate code | None introduced; `IdentityFunc` reused from `enterprise_handler.go`, context key reuse via `middleware.IdentityFromContext` |
| Unused code | None; `toMeResponse`, `MeResponse`, `MeProfileResponse` all used |
| Response shape | Matches contract exactly: `public_id`, `email`, `role`, `profile.full_name`; `profile: null` when absent (pointer with no `omitempty`) |
| Import-cycle workaround | `IdentityFunc` injection into `NewAuthHandler` avoids `handler → middleware → handler` cycle; same pattern as `enterprise_handler.go:18`. Correct, not a workaround smell — it is the established pattern |
| Route/middleware consistency | `GET /auth/me` registered behind `Authenticate` only (route.go:50), matching contract "Authenticated". No `RequireAdmin`/`RequireSelf` — correct for self-identity endpoint |
| API contract accuracy | Contract section 5, route permissions table line 41, and auth section bullet all added; subsequent section numbers renumbered (6–29) consistently. No drift from `route.go` |
| Test gaps | Covered: shape + leak assertions, DB role over JWT claim, null profile, missing identity 401, route registration. Not covered (acceptable): 401 via real middleware for `/auth/me` is covered indirectly by `TestMeRejectsMissingIdentity` through the full stack |
| Security | Role from DB identity, never JWT claim (enforced by test). No password/token/timestamp leakage (enforced by test). No new inputs to validate. No PII beyond email and full name, both already exposed to the authenticated self via `GET /users/{publicID}` for admins and login response |
| Unnecessary changes | None; every hunk is needed. Contract renumbering is required by inserting section 5 |
| Documentation/checklist quality | Contract complete and accurate; plan checklist honest; AGENTS.md needs no update (no new runtime constraint introduced) |

## Readiness

**Ready to merge** once the working tree is committed. No blocking findings. The two optional suggestions (no-op identity in `auth_handler_test.go`, nothing else) can be addressed or skipped; neither affects correctness or security.
