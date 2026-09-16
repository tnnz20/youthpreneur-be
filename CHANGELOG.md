# Changelog

## Unreleased

### Added

- Viper environment configuration.
- Configurable `log/slog` logging.
- `GET /healthz` endpoint.
- PostgreSQL 16 Alpine Compose service.
- Dockerfile and Makefile development commands.
- Go documentation comments for exported identifiers.

### Changed

- Centralized dependency wiring in `config.Bootstrap`.
- Renamed health use case identifiers to follow Go naming conventions.
