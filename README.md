# youthpreneur-be

Go HTTP service for Youthpreneur.

## Getting Started

### Requirements

- Go 1.27+
- Make
- Podman or Docker with Compose support

### Run locally

```bash
cp .env.example .env
go run ./cmd/web
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Response:

```json
{"status":"ok"}
```

### Run PostgreSQL

PostgreSQL is provided for local development. The application does not connect to PostgreSQL yet.

```bash
make compose-up
```

Use Docker instead of Podman:

```bash
make compose-up engine=docker
```

Stop containers:

```bash
make compose-down
```

Stop containers and remove volumes:

```bash
make compose-down-v
```

## Configuration

Configuration is read from environment variables through Viper.

| Variable | Default | Description |
| --- | --- | --- |
| `APP_ADDR` | `:8080` | HTTP listen address |
| `APP_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error` |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `youthpreneur` | PostgreSQL database |

`.env` is local-only and ignored by Git. Use `.env.example` as the starting template.

## Project Layout

```text
cmd/web/                         HTTP server entrypoint
internal/config/                 configuration, logger, and dependency bootstrap
internal/delivery/http/          HTTP handlers and routes
internal/entity/                 domain entities
internal/model/                  HTTP response models
internal/repository/             repository interfaces and implementations
internal/usecase/                application use cases
compose.yaml                     local PostgreSQL container
Dockerfile                       API container build
Makefile                         development commands
```

Request flow:

```text
HTTP request → route → handler → usecase → repository
```

## Commands

```bash
make run
make test
make vet
make fmt
make build
make compose-up
make compose-down
make compose-down-v
```

## Docker Build

```bash
docker build -t youthpreneur-be .
docker run --rm -p 8080:8080 youthpreneur-be
```

## License

No license has been declared yet.
