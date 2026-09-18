.DEFAULT_GOAL := help

engine ?= podman
compose := $(engine) compose

.PHONY: help run test vet fmt build seed migrate-up migrate-down migrate-force migrate-version compose-up compose-down compose-down-v

help:
	@echo "Go commands:"
	@echo "  run              Run the API from source"
	@echo "  test             Run Go tests"
	@echo "  vet              Run go vet"
	@echo "  fmt              Format Go sources"
	@echo "  build            Build bin/web"
	@echo "  seed             Seed the initial admin from SEEDER_ADMIN_*"
	@echo ""
	@echo "Migration commands:"
	@echo "  migrate-up       Apply database migrations"
	@echo "  migrate-down     Roll back all database migrations"
	@echo "  migrate-force    Force a version: make migrate-force version=1"
	@echo "  migrate-version  Print the current migration version"
	@echo ""
	@echo "Compose commands:"
	@echo "  compose-up       Start containers"
	@echo "  compose-down     Stop containers"
	@echo "  compose-down-v   Stop containers and remove volumes"

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
