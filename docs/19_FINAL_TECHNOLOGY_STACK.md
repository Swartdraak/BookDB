# Final Technology Stack and Architecture Decisions

## Decision summary

The stack is selected for a large, long-running, self-hosted metadata platform, not for minimum demo complexity.

## Backend — Go

**Decision:** Go 1.26 or newer supported minor line.

Why:
- excellent streaming/network concurrency;
- predictable memory model for bulk imports;
- static native binaries;
- easy Linux container builds and Windows/macOS developer builds;
- mature PostgreSQL, NATS, OpenSearch, S3, OpenTelemetry ecosystems;
- strong standard library HTTP stack;
- straightforward horizontal worker processes;
- low operational overhead compared with JVM/Node server stacks.

Current research baseline (2026-09-06): Go 1.26.8 was released 2026-09-01.

## Canonical database — PostgreSQL 18

**Decision:** PostgreSQL 18 supported minor line.

Why:
- transactional truth store;
- strong relational integrity;
- JSONB for raw/normalized source payload fragments;
- partitioning;
- mature replication/PITR;
- rich indexing;
- reliable migration ecosystem.

Current research baseline: PostgreSQL 18.6 is supported through November 2030.

### HA
Reference:
- Patroni;
- PostgreSQL streaming replication;
- etcd DCS;
- PgBouncer;
- load-balanced writer/read endpoints.

BookDB itself does not embed a PostgreSQL failover manager.

## Durable work/event bus — NATS JetStream

**Decision:** NATS + JetStream for durable tasks and internal event streams.

Why:
- FOSS Apache-2.0;
- Go-native ecosystem;
- persisted/replayable messages;
- durable consumers;
- replicated streams;
- RAFT-backed cluster;
- 3 or 5 JetStream servers recommended for HA;
- supports horizontal worker fleets without turning BookDB into a microservice maze.

Delivery semantics:
- assume at-least-once;
- explicit acknowledgements;
- idempotency keys;
- bounded retries;
- dead-letter/quarantine streams;
- outbox events are deduplicated.

## Cache — Valkey

**Decision:** Valkey 9.x.

Purpose:
- read cache;
- API rate-limit counters;
- short-lived distributed mutexes where safe;
- ephemeral expensive-query memoization.

Not authoritative:
- canonical metadata;
- job truth;
- publication history.

HA modes:
- Sentinel for simpler primary/replica topology;
- Valkey Cluster where sharding is useful.

Current research baseline: Valkey 9.1.2 is current as of 2026-09-01.

## Search — OpenSearch

**Decision:** OpenSearch 3.x production search backend.

Why:
- distributed;
- Apache-2.0;
- replicas/shards;
- multilingual/fuzzy/search-analysis support;
- operationally appropriate for tens/hundreds of millions of searchable documents.

Reference distributed minimum: 3 cluster-manager-eligible nodes, with data roles sized separately when catalog size requires.

PostgreSQL search remains a dev/recovery fallback.

## Object storage — S3 abstraction with SeaweedFS reference implementation

**Decision:** BookDB uses S3 API semantics internally; it does not bind to one storage vendor.

Reference FOSS deployment:
- SeaweedFS 4.x;
- Apache-2.0;
- distributed S3-compatible storage;
- designed to scale to very large object counts.

Other supported targets:
- Ceph RGW;
- AWS S3;
- compatible cloud object stores;
- existing operator-managed S3 systems.

**MinIO is no longer the default recommendation** because its upstream open-source repository was archived in April 2026. Existing compatible deployments can still be supported through S3.

## Frontend

- React 19.2+
- TypeScript
- Vite 8.1+
- TanStack Query v5
- TanStack Table
- OpenAPI-generated client
- accessible component primitives selected for WCAG 2.2 AA compliance
- Playwright for E2E
- Vitest for frontend tests

Create React App is not used.

## Authentication

- secure local accounts;
- OIDC federation;
- group/claim-to-role mappings;
- emergency local Administrator account can be retained;
- OIDC configuration per instance;
- API keys for machine integrations;
- future OAuth device/client credential flows may be added without replacing API keys.

## Observability

- OpenTelemetry SDK/instrumentation;
- Prometheus/OpenMetrics endpoint;
- structured JSON logs;
- trace propagation across NATS messages;
- Grafana is optional operator tooling, not an application dependency.

## Packaging

BookDB server:
- OCI images
- native binaries
- source tarballs

JavaScript:
- `@bookdb/sdk` — Apache-2.0
- `@bookdb/cli` — Apache-2.0 or AGPL depending on whether it contains server-derived code; keep SDK permissive.
- optional `npx bookdb-init` bootstrap tool.

The BookDB server itself will not be an npm package.
