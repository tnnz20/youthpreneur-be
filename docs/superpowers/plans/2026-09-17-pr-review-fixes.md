# PR Review Fixes Implementation Plan

**Goal:** Address approved PR review findings while leaving unauthenticated password reset and valid PostgreSQL migration SQL unchanged.

## Scope

- Skip finding 1: password reset remains temporarily unauthenticated until auth feature.
- Skip finding 8: no code issue; repository uses PostgreSQL syntax.
- Ignore client-provided admin role; new users always use `member`.
- Limit JSON request bodies to 1 MiB.
- Return safe stable validation messages; keep wrapped details internal/logged.
- Keep six-digit public ID format; handle collision exhaustion clearly and document it is not an auth credential.
- Reject passwords longer than bcrypt's 72-byte limit.
- Keep `user_role` enum; future roles require a new migration and application validation update.
- Ensure shutdown cleanup runs before process exit.
- Reject future birth dates.
- Make password change atomic against current password hash.
- Make migration DSN conversion return errors instead of silently falling back.

## Tasks

1. Update user creation to ignore `request.Role` and default role to member.
2. Add `http.MaxBytesReader` in shared request decoding and test oversized bodies.
3. Add `usecase.ErrBadRequest` and map validation errors to safe HTTP 400 messages without exposing internal sentinel prefixes.
4. Preserve public ID format and add explicit collision exhaustion handling/documentation.
5. Enforce bcrypt maximum input length in bytes with tests.
6. Replace shutdown `os.Exit` paths with cleanup-safe return flow.
7. Validate birth date is not after current date with tests.
8. Add atomic repository password-change operation using current hash predicate; map zero affected rows to invalid credentials; update usecase/tests.
9. Change migration DSN helper to return `(string, error)` and test parse failure handling.
10. Update README and CHANGELOG for intentional unauthenticated reset, role behavior, limits, and future-role migration process.
11. Run `gofmt -w .`, `go test ./...`, `go vet ./...`, and `git diff --check`.

## Acceptance

- Existing tests pass.
- New tests cover role override, body limit, safe errors, password length, future birth date, atomic password change, and DSN errors.
- No auth middleware added.
- No migration schema changes added.
- No commit or push unless separately requested.
