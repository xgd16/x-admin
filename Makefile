SHELL := /bin/sh
GO ?= go
NPM ?= npm

.PHONY: run build test coverage coverage-html lint fmt frontend frontend-build \
	frontend-test test-all dev-infra dev-infra-down compose-up compose-down compose-logs tidy wire

run:
	$(GO) run ./cmd/server -config configs/config.yaml

build:
	$(GO) build -trimpath -o bin/x-admin ./cmd/server

test:
	$(GO) test ./...

frontend-test:
	cd frontend && $(NPM) test

test-all: test frontend-test

coverage:
	$(GO) test ./tests/... -coverpkg=./internal/... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

coverage-html: coverage
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "open coverage.html"

lint:
	$(GO) vet ./...
	cd frontend && $(NPM) run lint

fmt:
	gofmt -w cmd internal migrations tests tool

tidy:
	$(GO) mod tidy

wire:
	$(GO) run github.com/google/wire/cmd/wire@latest ./internal/bootstrap

frontend:
	cd frontend && $(NPM) run dev

frontend-build:
	cd frontend && $(NPM) run build

dev-infra:
	docker compose -f deploy/docker-compose.dev.yml up -d

dev-infra-down:
	docker compose -f deploy/docker-compose.dev.yml down

compose-up:
	docker compose -f deploy/docker-compose.yml up -d --build

compose-down:
	docker compose -f deploy/docker-compose.yml down

compose-logs:
	docker compose -f deploy/docker-compose.yml logs -f --tail=200
