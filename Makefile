.DEFAULT_GOAL := help

engine ?= podman
compose := $(engine) compose
ssh_flag := $(if $(filter true 1,$(ssh)),--ssh,)

.PHONY: help run test vet fmt build seed seed-catalog seed-users migrate-up migrate-down migrate-force migrate-version compose-up compose-down compose-down-v

help:
	@echo "Go commands:"
	@echo "  run              Run the API from source"
	@echo "  test             Run Go tests"
	@echo "  vet              Run go vet"
	@echo "  fmt              Format Go sources"
	@echo "  build            Build bin/web"
	@echo "  seed             Seed the initial admin from SEEDER_ADMIN_*: make seed [ssh=true]"
	@echo "  seed-catalog     Seed training catalogs: make seed-catalog [file=path/to.csv] [ssh=true]"
	@echo "  seed-users       Seed users and enterprises: make seed-users [file=path/to.csv] [ssh=true]"
	@echo ""
	@echo "Migration commands:"
	@echo "  migrate-up       Apply database migrations: make migrate-up [ssh=true]"
	@echo "  migrate-down     Roll back all database migrations: make migrate-down [ssh=true]"
	@echo "  migrate-force    Force a version: make migrate-force version=1 [ssh=true]"
	@echo "  migrate-version  Print the current migration version: make migrate-version [ssh=true]"
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
	go run ./cmd/seeder $(ssh_flag)

seed-catalog:
	go run ./cmd/seed-catalog $(if $(file),-file $(file)) $(ssh_flag)

seed-users:
	go run ./cmd/seed-users $(if $(file),-file $(file)) $(ssh_flag)

migrate-up:
	go run ./cmd/migrate $(ssh_flag) up

migrate-down:
	go run ./cmd/migrate $(ssh_flag) down

migrate-force:
	go run ./cmd/migrate $(ssh_flag) force $(version)

migrate-version:
	go run ./cmd/migrate $(ssh_flag) version

compose-up:
	$(compose) up -d

compose-down:
	$(compose) down

compose-down-v:
	$(compose) down -v
