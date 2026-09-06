# Architecture and Design Documentation

## Architectural style

BookDB uses a **modular distributed application in one repository**.

This replaces the earlier “single modular monolith first” assumption because the confirmed scope requires horizontal workers and HA from the beginning.

The project avoids unnecessary microservice fragmentation:
- shared domain modules;
- one version/release train;
- multiple executable roles;
- durable NATS contracts between asynchronous stages;
- PostgreSQL remains the single canonical write model.

## Core layers

### Edge
- reverse proxy/load balancer
- WebUI
- REST API
- authentication/API keys

### Catalog services
- catalog queries
- proposals/admin decisions
- merge/split
- provenance
- API DTOs

### Processing
- scheduler
- ingestion
- normalization
- identity resolution
- reconciliation
- canonical publication
- indexing
- assets

### Data
- PostgreSQL HA
- NATS JetStream
- OpenSearch
- SeaweedFS/S3
- Valkey

## Canonical write pattern

All canonical mutations occur in PostgreSQL transactions.

After a canonical mutation:
1. outbox row is created in same transaction;
2. outbox publisher sends event to JetStream;
3. subscribers update search/cache/change feed;
4. idempotency prevents duplicate side effects.

## Availability behavior

### PostgreSQL unavailable
Public API may serve explicitly safe cached responses where configured, but writes/admin/ingestion pause. No split-brain fallback database is created.

### OpenSearch unavailable
Exact ID endpoints remain available from PostgreSQL; general search is degraded/unavailable. Index can be rebuilt later.

### NATS unavailable
API read paths continue; new async jobs/syncs pause rather than silently execute without durable queueing.

### Valkey unavailable
Performance/rate-limit strategy degrades to configured safe fallback; metadata remains correct.

### Object store unavailable
Metadata remains queryable; assets return unavailable/degraded response.

## Data partitioning

Start with PostgreSQL native partitioning for high-volume append/history tables:
- source_record
- claim history
- audit_event
- outbox/event history
- ingestion records

Partition key chosen by measured access pattern, likely source/time/hash combinations. Partitioning strategy is benchmarked before production bootstrap.

## Search indexing

OpenSearch index documents are denormalized projections:
- work search document;
- edition search document;
- person search document;
- series search document.

Versioned index aliases allow zero/low-downtime rebuild.

## Security boundaries

- connector egress allowlists;
- source credentials isolated;
- admin mutations server-authorized;
- user proposal content untrusted;
- NATS subjects use account/permission policy;
- database application role is not superuser;
- workers have least-privilege service identities.

## Portability

Development Compose is the common denominator. IDEs do not own build state. All operations have terminal equivalents.
