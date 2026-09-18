# PR #5 Review: Admin Seeder and Member User Listing

## Verification

- `go test -count=1 ./...`: passed.
- `go test -race -count=1 ./...`: passed.
- `go vet ./...`: passed.
- PR-touched Go files formatted; unrelated pre-existing formatting noise exists on master.
- `git diff --check`: passed.
- PostgreSQL integration tests were skipped because `TEST_POSTGRES_DSN` was unavailable.

## Checklist

- Environment-driven `cmd/seeder`: pass.
- Credentials validated before DB connection: pass.
- User/profile creation is transactional: pass.
- Duplicate email returns an error without partial writes: pass.
- Password is bcrypt-hashed and never logged: pass.
- Generated public ID uses existing `YTP-` format with collision retries: pass.
- Default admin profile is applied: pass.
- `GET /users` excludes admins before cursor/limit: pass.
- `GET /users/{publicID}` still supports direct active-admin lookup: pass.
- Cursor pagination remains gap-free: pass.
- Makefile and README seeder command documented: pass.

## Medium Findings

### M1. API contract does not document admin exclusion

Location: `api/api-contract.md:261`

Contract still says “List active users,” but `GET /users` now returns active member users only and excludes admins.

**Suggestion:** Change wording to “List active member users” and state that admin accounts are excluded. Mention direct admin lookup through `GET /users/{publicID}`.

## Low Findings

- `AGENTS.md` does not document seeder variables or member-only list behavior.
- `internal/usecase/user_usecase.go:447-483` has exported wrappers alongside underlying unexported helpers. Consider consolidating to avoid duplicate wrapper/implementation pairs.
- `cmd/seeder/main.go:35` uses a package-level mutable `adminProfile`; make it local or return it from a function.
- Structural SQL string-position test is brittle; DSN-backed integration test is the stronger behavior check.
- Integration tests were skipped in this environment; run them with migrated PostgreSQL and `TEST_POSTGRES_DSN` before merge.
- README could advise rotating the seeded admin password after provisioning.
- Seeder reads process environment only; `.env` values are not automatically loaded. This matches existing command behavior and is documented through exported env usage.

## Duplicate and Unused Code

No critical unused production code found. Existing `CreateUser` transaction is reused correctly instead of duplicating SQL. Minor duplicate wrapper/implementation pairs in `user_usecase.go` are candidates for cleanup.

## Security and Transaction Review

No critical/high security issue found. Seeder validates credentials before DB connection, hashes passwords, keeps secrets out of logs, uses bounded public-ID retries, and relies on the existing transactional user/profile repository method.

## Conclusion

PR #5 correctly adds admin provisioning and member-only user listing. Core behavior, security, transaction handling, pagination, and tests are sound. Main gap is API documentation drift.

## Merge Recommendation

**Approve with changes:** update `api/api-contract.md` to document admin exclusion from `GET /users`. Other findings are non-blocking; run PostgreSQL integration tests before merge if CI does not provide them.
