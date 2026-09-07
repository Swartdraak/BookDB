# System Documentation

## Deployable roles

- API/Web
- Scheduler
- Ingest Worker
- Normalize Worker
- Identity Worker
- Reconcile Worker
- Publish Worker
- Search Indexer
- Asset Worker
- Maintenance Worker

All are built from one Go repository and share domain packages.

## Required production services

- PostgreSQL 18
- PgBouncer
- NATS JetStream
- OpenSearch 3.x
- S3-compatible object store
- Valkey 9

## HA reference services

- Patroni
- etcd
- multiple BookDB API/worker processes
- replicated NATS/OpenSearch/S3/cache

## Data flow

```text
Schedule/manual trigger
  -> NATS sync request
  -> connector worker
  -> raw source record
  -> normalized claims
  -> identity resolution
  -> reconciliation
  -> PostgreSQL canonical transaction
  -> transactional outbox
  -> canonical.changed
      -> OpenSearch index
      -> change feed/cache invalidation
```

## Truth hierarchy

PostgreSQL is authoritative for:
- entities;
- claims;
- provenance;
- proposals;
- admin decisions;
- audit;
- source policy/config metadata;
- durable schedule/checkpoint records.

NATS is authoritative only for in-flight/replayable event streams within configured retention.
OpenSearch is a projection.
Valkey is ephemeral.
Object store is authoritative for retained object bytes but object metadata/rights/checksum live in PostgreSQL.
