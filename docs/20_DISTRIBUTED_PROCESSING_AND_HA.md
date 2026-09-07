# Distributed Processing, Horizontal Scaling, and High Availability

## Goal

BookDB must support the scale of a global bibliographic catalog without assuming a single process or single host.

## Process roles

One source repository can build the following roles:

- `bookdb api`
- `bookdb scheduler`
- `bookdb worker ingest`
- `bookdb worker normalize`
- `bookdb worker identity`
- `bookdb worker reconcile`
- `bookdb worker publish`
- `bookdb worker index`
- `bookdb worker assets`
- `bookdb worker maintenance`

Roles share domain libraries but communicate through durable messages and canonical DB state.

## NATS subjects/streams

Suggested logical subjects:

```text
bookdb.source.sync.requested
bookdb.source.record.discovered
bookdb.source.record.fetched
bookdb.claims.normalized
bookdb.identity.resolve
bookdb.reconcile.requested
bookdb.canonical.changed
bookdb.search.index
bookdb.asset.fetch
bookdb.asset.process
bookdb.maintenance.*
bookdb.deadletter.*
```

## Job contract

Every durable job has:
- job ID (UUIDv7);
- operation type/version;
- source/entity scope;
- idempotency key;
- creation time;
- attempt;
- trace context;
- priority;
- payload schema version.

Worker behavior:
1. receive;
2. validate schema;
3. acquire domain-safe idempotency guard;
4. execute transaction;
5. commit;
6. acknowledge.

Crashes before acknowledgement cause redelivery. Handlers must be safe under duplicate delivery.

## Transactional outbox

Canonical DB transaction writes:
- entity/claim changes;
- audit data;
- outbox record.

A publisher process atomically claims unsent outbox rows and publishes them to NATS. Only after confirmed publish is the outbox row marked sent.

This prevents:
- database changed but event lost;
- event emitted before database commit.

## Database HA

Reference topology:

```text
                   PgBouncer/HAProxy
                      /       \
              write endpoint  read endpoint
                    |
                Patroni leader
                /           \
          sync/async        standby
           standby
                |
           WAL archive
```

Patroni uses etcd/other supported DCS for leader coordination.

Requirements:
- no application superuser;
- replication slots monitored;
- WAL archive configured;
- PITR tested;
- split-brain protection/fencing documented;
- automatic failover tested quarterly in reference deployment;
- migrations run with leader awareness.

## NATS HA

Production recommendation:
- 3 or 5 JetStream-enabled servers;
- file-backed JetStream storage on local reliable SSD;
- stream replication factor 3 for critical task/event streams;
- no shared NFS JetStream data directory;
- quorum health monitored.

## OpenSearch HA

Reference:
- minimum three cluster-manager-eligible nodes for real failover;
- replica shards for catalog indexes;
- index templates/versioned aliases;
- blue/green index rebuild:
  1. create `catalog-vN`;
  2. fill;
  3. validate;
  4. atomically move alias;
  5. retain previous index for rollback window.

## Object storage HA

BookDB only depends on S3 semantics.

Reference SeaweedFS topology should separate:
- masters/coordination;
- volume servers;
- filer/S3 gateway;
- replicated/erasure-coded storage according to operator durability policy.

Object database rows store object key/checksum/rights metadata; object store is not used for relational truth.

## Valkey HA

Valkey loss must degrade performance, not corrupt truth.

Use:
- Sentinel where simple HA cache is sufficient;
- Cluster where sharding is required.

Cache stampede controls must survive cache loss by falling back to DB/search.

## Scheduler HA

Multiple schedulers may run, but only one logical schedule execution for `(source, schedule_window)` can be emitted.

Use PostgreSQL advisory/lease table or JetStream KV/consumer semantics with:
- lease expiration;
- idempotency key;
- unique DB constraint on schedule execution.

Do not rely on a single always-running leader process.

## Multi-region

Not required for v1, but architecture must not preclude:
- read-only API/search replicas in another region;
- replicated object storage;
- NATS supercluster/source streams;
- PostgreSQL DR standby.

Multi-primary canonical writes are explicitly not a v1 goal.
