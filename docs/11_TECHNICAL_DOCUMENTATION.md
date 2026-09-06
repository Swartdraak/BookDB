# Technical Documentation

## Repository target

```text
cmd/bookdb/
internal/
  api/
  auth/
  catalog/
  claims/
  proposals/
  publication/
  identity/
  reconcile/
  scheduler/
  ingestion/
  sources/
  assets/
  search/
  events/
  outbox/
  audit/
  observability/
web/
migrations/
api/
schemas/
deployment/
docs/
agent-kit/
```

## Commands

- `bookdb api`
- `bookdb scheduler`
- `bookdb worker ingest`
- `bookdb worker normalize`
- `bookdb worker identity`
- `bookdb worker reconcile`
- `bookdb worker publish`
- `bookdb worker index`
- `bookdb worker assets`
- `bookdb migrate`
- `bookdb doctor`
- `bookdb source list`
- `bookdb source sync <source> [--full]`
- `bookdb source pause <source>`
- `bookdb source resume <source>`
- `bookdb reconcile --scope ...`
- `bookdb search reindex`
- `bookdb jobs retry`
- `bookdb version`

## Baseline toolchain

- Go 1.26+
- PostgreSQL 18
- Node LTS compatible with Vite 8
- pnpm for frontend workspace
- React 19.2+
- Vite 8.1+
- Docker/Compose
- OpenAPI generator
- golangci-lint
- Vitest/Playwright
- integration tests with real containers

## Configuration

Config:
defaults < YAML config < environment < secret references/runtime.

Secret values are never included in generated diagnostics.

## Event schemas

Every NATS payload is versioned:
`bookdb.events.<domain>.v1`.

Schema compatibility is tested in CI.

## Migrations

Forward-only after release.
Large migrations use expand/migrate/contract.
Reconciliation algorithm changes are versioned separately from database migrations.
