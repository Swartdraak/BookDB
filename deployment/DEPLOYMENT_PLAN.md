# Deployment Plan — Final

## Supported deployment modes

### Docker Compose
Official for:
- development;
- single-host standard deployment;
- HA functional testing.

### Direct OCI
Each BookDB role can run as an individual container connected to external services.

### Native binary
Supported for BookDB application roles on Linux, Windows, and macOS. Production stateful dependencies remain separately managed.

### Multi-host distributed production
Same OCI images can run under:
- Kubernetes;
- Nomad;
- Docker Swarm;
- other container schedulers;
- manual service managers,
provided required HA/service contracts are met.

BookDB does not force Kubernetes.

## Standard Compose services

- api
- scheduler
- worker-ingest
- worker-reconcile
- worker-index
- postgres
- pgbouncer
- nats
- valkey
- opensearch
- seaweedfs

Worker counts are configurable.

## Distributed topology

See `docs/20_DISTRIBUTED_PROCESSING_AND_HA.md` and `docs/25_DEPLOYMENT_PROFILES.md`.

## npm

Not a server deployment.

Publish:
- `@bookdb/sdk`
- optional `@bookdb/cli`
- optional `npx bookdb-init`

## Cross-platform development

- `.devcontainer` recommended;
- native Go/Node toolchains supported;
- PowerShell and POSIX shell wrappers where scripts are required;
- no bash-only mandatory build flow;
- VS Code and JetBrains configurations are optional conveniences.
