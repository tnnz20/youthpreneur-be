# Runtime Configuration and Containers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Viper-based environment configuration, configurable `log/slog` logging, PostgreSQL container files, and Makefile commands.

**Architecture:** Viper owns configuration defaults and environment lookup. Bootstrap receives configuration and logger dependencies, while `cmd/web/main.go` remains process startup only. Docker runs the API separately from PostgreSQL; no database client or connection is added yet.

**Tech Stack:** Go, Viper, `log/slog`, Docker/Podman Compose, Make.

**Spec:** User-approved plan in conversation.

## Global Constraints

- Use Viper for environment configuration.
- Use `log/slog` for application logging.
- Use only `postgres:16-alpine` for PostgreSQL container.
- Use lowercase Makefile variable `engine`, default `podman`.
- Support `make compose-<target> engine=docker`.
- Do not add PostgreSQL driver or database connection.
- Do not add `compose-up` or `compose-down` targets; use `compose-up`, `compose-down`, and `compose-down-v` as approved names.
- Keep `.env` ignored and commit `.env.example`.

---

### Task 1: Configuration and Logger

**Files:**
- Modify: `go.mod`
- Modify: `internal/config/config.go`
- Create: `internal/config/logger.go`
- Modify: `internal/config/bootstrap.go`
- Modify: `cmd/web/main.go`
- Create: `.env.example`

**Interfaces:**
- `config.Load() Config` reads `APP_ADDR`, `APP_LOG_LEVEL`, and `POSTGRES_*` values through Viper.
- `config.NewLogger(level string) *slog.Logger` returns a text-handler logger using configured level.
- `config.Bootstrap(logger *slog.Logger) *http.ServeMux` builds current routes.

- [ ] Add Viper dependency with `go get github.com/spf13/viper`.
- [ ] Configure Viper defaults: `app.addr=:8080`, `app.log_level=info`, PostgreSQL host `localhost`, port `5432`, user `postgres`, password `postgres`, database `youthpreneur`.
- [ ] Configure environment mapping with prefix `APP` for app values and explicit `POSTGRES_*` bindings; use `AutomaticEnv`, key replacer, and typed getters.
- [ ] Extend `Config` with address, log level, and PostgreSQL fields.
- [ ] Implement `NewLogger` with `slog.LevelDebug`, `slog.LevelInfo`, `slog.LevelWarn`, and `slog.LevelError`; unknown values use info.
- [ ] Pass logger into bootstrap and replace `log.Printf`/`log.Fatal` in `main.go` with `slog` calls and `http.Server` startup.
- [ ] Add `.env.example` containing all supported variables without secrets.
- [ ] Run `gofmt -w cmd internal`, `go test ./...`, and `go vet ./...`.

### Task 2: Container Files

**Files:**
- Create: `Dockerfile`
- Create: `compose.yaml`

**Interfaces:**
- `Dockerfile` builds and runs `cmd/web`.
- Compose service `postgres` uses image `postgres:16-alpine`, persistent named volume, healthcheck, and environment variables.

- [ ] Use a multi-stage Go build with module download, build `/app/web`, and minimal runtime image.
- [ ] Expose API port `8080`.
- [ ] Define PostgreSQL environment values matching `.env.example`.
- [ ] Add named volume for PostgreSQL data and healthcheck using `pg_isready`.
- [ ] Keep Compose free of API orchestration requirements beyond the PostgreSQL service.

### Task 3: Makefile

**Files:**
- Create: `Makefile`

**Interfaces:**
- `make run` runs `go run ./cmd/web`.
- `make test` runs `go test ./...`.
- `make vet` runs `go vet ./...`.
- `make fmt` runs `gofmt -w cmd internal`.
- `make build` builds `bin/web`.
- `make compose-up`, `make compose-down`, and `make compose-down-v` invoke `$(engine) compose ...`.
- `engine ?= podman`; `make compose-up engine=docker` selects Docker.

- [ ] Add `.DEFAULT_GOAL := help` and targets for run, test, vet, fmt, build.
- [ ] Define lowercase `engine ?= podman` and use `compose := $(engine) compose`.
- [ ] Add Compose targets exactly as approved.
- [ ] Add concise help output without adding extra targets.
- [ ] Run `make test`, `make vet`, and `make build`.

### Task 4: Final Verification

**Files:**
- Verify all changed files.

- [ ] Run `gofmt -l .` and require no output.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `git diff --check`.
- [ ] Inspect `git status` and summarize changed files.
