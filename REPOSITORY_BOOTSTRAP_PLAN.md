# Repository Bootstrap Plan

## Root

- `LICENSE`
- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `CODE_OF_CONDUCT.md`
- `go.mod`
- `Makefile` or cross-platform task definition
- `compose.yaml`
- `.env.example`
- `.editorconfig`
- `.gitignore`

## Application

- `cmd/bookdb`
- `internal/*`
- `web/`
- `api/`
- `migrations/`
- `schemas/`

## Operations

- `deployment/compose/`
- `deployment/ha/`
- `deployment/kubernetes/` optional reference
- `monitoring/`
- `scripts/`

## Governance

- `docs/`
- `adrs/`
- `agent-kit/`
- `.github/`

## First CI milestone

A clean checkout must be able to:
- build Go binaries;
- build WebUI;
- start development Compose;
- migrate empty DB;
- run unit/integration tests;
- validate OpenAPI;
- validate source registry schema.
