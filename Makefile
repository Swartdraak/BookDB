SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

.DEFAULT_GOAL := help

GO ?= go
COMPOSE ?= docker compose
OPENAPI_SPEC ?= api/openapi-outline.yaml
GO_FILES := $(shell find . -type f -name '*.go' \
	-not -path './.git/*' \
	-not -path './.agent-state/*' \
	-not -path './.codex/*' \
	-not -path './vendor/*' \
	-not -path './bin/*' \
	-not -path './web/node_modules/*' \
	-not -path './web/dist/*')
GO_PACKAGES := $(shell $(GO) list ./... | grep -vE '(/web/node_modules/|/\.agent-state/|/\.codex/)(|$$)')

.PHONY: help setup fmt format fmt-check lint test build openapi-validate compose-up compose-down compose-reset compose-app-build compose-app-up integration ci

help:
	@printf '%s\n' 'BookDB root targets:' \
		'  make setup             Download Go modules' \
		'  make fmt               Format Go source in place' \
		'  make fmt-check         Verify Go source formatting' \
		'  make lint              Run Go vet' \
		'  make test              Run Go tests' \
		'  make build             Build all Go packages' \
		'  make compose-app-build Build the BookDB app image' \
		'  make openapi-validate  Validate api/openapi-outline.yaml' \
		'  make compose-up        Start the development compose stack' \
		'  make compose-app-up    Start the full development compose stack' \
		'  make compose-down      Stop the development compose stack (preserves volumes)' \
		'  make compose-reset     Stop the stack AND delete its volumes (disposable reset)' \
		'  make integration       Run the compose smoke test' \
		'  make ci                Run the local CI gate sequence'

setup:
	$(GO) mod download

fmt:
	gofmt -w $(GO_FILES)

format: fmt

fmt-check:
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [[ -n "$$files" ]]; then \
		printf '%s\n' "$$files"; \
		exit 1; \
	fi

lint:
	$(GO) vet $(GO_PACKAGES)

test:
	$(GO) test $(GO_PACKAGES)

build:
	$(GO) build $(GO_PACKAGES)

openapi-validate:
	$(GO) run ./scripts/dev/validate-openapi.go $(OPENAPI_SPEC)

compose-up:
	$(COMPOSE) -f compose.yaml up -d --wait --wait-timeout 180

compose-app-build:
	$(COMPOSE) -f compose.yaml -f deployment/compose/app.dev.yaml build

compose-app-up:
	$(COMPOSE) -f compose.yaml -f deployment/compose/app.dev.yaml up -d --wait --wait-timeout 180

compose-down:
	$(COMPOSE) -f compose.yaml down --remove-orphans

compose-reset:
	$(COMPOSE) -f compose.yaml down -v --remove-orphans

integration:
	bash scripts/dev/compose-smoke.sh

ci: fmt-check lint test build openapi-validate
