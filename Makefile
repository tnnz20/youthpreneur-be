.DEFAULT_GOAL := help

engine ?= podman
compose := $(engine) compose

.PHONY: help run test vet fmt build compose-up compose-down compose-down-v

help:
	@echo Targets:
	@echo "  run             Run the API from source"
	@echo "  test            Run Go tests"
	@echo "  vet             Run go vet"
	@echo "  fmt             Format Go sources"
	@echo "  build           Build bin/web"
	@echo "  compose-up      Start containers"
	@echo "  compose-down    Stop containers"
	@echo "  compose-down-v  Stop containers and remove volumes"

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

compose-up:
	$(compose) up -d

compose-down:
	$(compose) down

compose-down-v:
	$(compose) down -v
