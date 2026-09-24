.DEFAULT_GOAL := help

engine ?= podman
compose := $(engine) compose

.PHONY: help run test vet fmt build seed seed-catalog migrate-up migrate-down migrate-force migrate-version compose-up compose-down compose-down-v

help:
	@echo "Go commands:"
	@echo "  run              Run the API from source"
	@echo "  test             Run Go tests"
	@echo "  vet              Run go vet"
	@echo "  fmt              Format Go sources"
	@echo "  build            Build bin/web"
	@echo "  seed             Seed the initial admin from SEEDER_ADMIN_*"
	@echo "  seed-catalog     Seed training catalogs: make seed-catalog [file=path/to.csv]"
	@echo ""
	@echo "Migration commands:"
	@echo "  migrate-up       Apply database migrations"
	@echo "  migrate-down     Roll back all database migrations"
	@echo "  migrate-force    Force a version: make migrate-force version=1"
	@echo "  migrate-version  Print the current migration version"
	@echo ""
	@echo "Compose commands (default engine=podman; override with engine=docker):"
	@echo "  compose-up       Start containers: make compose-up [engine=docker]"
	@echo "  compose-down     Stop containers: make compose-down [engine=docker]"
	@echo "  compose-down-v   Stop containers and remove volumes: make compose-down-v [engine=docker]"

run:
	go run ./cmd/web

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

build:
	go build -o bin/web ./cmd/web

seed:
	go run ./cmd/seeder

seed-catalog:
	go run ./cmd/seed-catalog $(if $(file),-file $(file))

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-force:
	go run ./cmd/migrate force $(version)

migrate-version:
	go run ./cmd/migrate version

compose-up:
	$(compose) up -d

compose-down:
	$(compose) down

compose-down-v:
	$(compose) down -v
