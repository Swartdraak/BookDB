# Technical Documentation

## Repository layout

The M0 foundation branch currently centers on:

```text
compose.yaml
deployment/compose/
config/
internal/config/
docs/
adrs/
.github/
project-management/
go.mod
```

The runtime trees `cmd/`, `web/`, `migrations/`, and `schemas/` are reserved by the architecture docs, but they are not yet part of this branch.

## Target command surfaces

The architecture docs describe the eventual BookDB process roles and CLI vocabulary, but this branch does not yet ship the `cmd/bookdb` executable tree. Treat the following as the target runtime surface, not as already-implemented commands:

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

## Configuration reference

Configuration precedence is:

`defaults` < YAML config file < environment variables < secret/runtime references.

The typed config package in `internal/config/` currently exposes these top-level sections:

- `env`
- `log`
- `api`
- `database`
- `nats`
- `valkey`
- `opensearch`
- `s3`
- `auth`
- `sources`
- `observability`
- `feature_flags`

Development defaults live in code, while `.env.example` and `config/source-registry.example.yaml` provide the local entry points for the root compose stack. Secret values are referenced, not inlined, in production-style configuration.

## Validation

- `go test ./internal/config` validates the typed config package and its precedence/redaction rules.
- Secret values are never included in generated diagnostics.

## Event and migration assumptions

- Every NATS payload is versioned.
- Schema compatibility is tested in CI once the product workflow lands.
- Forward-only migrations remain the release norm.
- Large migrations use expand/migrate/contract.
- Reconciliation algorithm changes are versioned separately from database migrations.
