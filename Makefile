# hashdupes developer tasks.
# Requires: Go toolchain, Node/npm, and (for dev/build) the Wails CLI.

GOPATH_BIN := $(shell go env GOPATH)/bin
WAILS      := $(GOPATH_BIN)/wails
MIGRATE    := go run ./cmd/migrate

.DEFAULT_GOAL := help

## help: show this help
.PHONY: help
help:
	@echo "hashdupes make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## tools: install the Wails CLI (matches the pinned wails/v2 version)
.PHONY: tools
tools:
	go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

## deps: install frontend dependencies
.PHONY: deps
deps:
	cd frontend && npm install

## dev: run the app with hot reload (Wails dev server)
.PHONY: dev
dev:
	$(WAILS) dev

## build: production build (creates build/bin/hashdupes)
.PHONY: build
build:
	$(WAILS) build -clean

## build-debug: debug build with devtools enabled
.PHONY: build-debug
build-debug:
	$(WAILS) build -debug -devtools

## test: run all Go tests
.PHONY: test
test:
	go test ./...

## test-race: run Go tests with the race detector
.PHONY: test-race
test-race:
	go test -race ./...

## vet: run go vet
.PHONY: vet
vet:
	go vet ./...

## fmt: format Go code
.PHONY: fmt
fmt:
	go fmt ./...

## check-frontend: type-check the Svelte/TS frontend
.PHONY: check-frontend
check-frontend:
	cd frontend && npm run check

## migrate-up: apply all pending database migrations
.PHONY: migrate-up
migrate-up:
	$(MIGRATE) up

## migrate-down: roll back the most recent migration
.PHONY: migrate-down
migrate-down:
	$(MIGRATE) down

## migrate-status: show migration status
.PHONY: migrate-status
migrate-status:
	$(MIGRATE) status

## fixture: build a synthetic test tree at /tmp/hashdupes-fixture
.PHONY: fixture
fixture:
	./scripts/mkfixture.sh /tmp/hashdupes-fixture --force

## doctor: check Wails system dependencies
.PHONY: doctor
doctor:
	$(WAILS) doctor

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -rf build/bin frontend/dist
