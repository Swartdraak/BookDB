# Metrics Documentation

## Data-quality

- canonical provenance coverage: target 100%
- false-merge rate
- false-split rate
- duplicate candidates per million entities
- invalid identifiers
- orphan relationships
- unresolved series conflicts
- field completeness by entity/language
- source freshness
- user proposal approval/rejection age
- restricted-source leakage: target 0
- assets with known rights/attribution

## Processing

- JetStream consumer lag
- redelivery count
- dead-letter count
- worker throughput
- outbox oldest-unsent age
- scheduler duplicate-dispatch prevention
- source sync duration
- source checkpoint lag
- reconciliation queue age
- index queue age

## API/search

- availability
- p50/p95/p99 latency
- 4xx/5xx rates
- search p95
- exact-ID p95
- cache hit rate
- OpenSearch rejected/search thread metrics

## HA

- PostgreSQL replication lag
- Patroni cluster state/failovers
- etcd quorum health
- NATS cluster/stream replica health
- OpenSearch cluster health
- S3 capacity/replication health
- Valkey replication/cluster health

## Initial SLOs

- API availability: 99.9%
- canonical ID p95: <150 ms on reference system
- search p95: <500 ms on reference system
- HTTP 5xx: <0.1%
- critical outbox age p95: <30 s under healthy system
- critical worker queue recovery without message loss after process restart
- documented default RPO <=24h; HA operators can target much lower using WAL/PITR
- restore procedure RTO target <=4h for reference deployment

Reference hardware/dataset must accompany performance claims.
