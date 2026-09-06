# Deployment Profiles

## Profile A — Developer Compose

Purpose: coding/tests on Windows/Linux/macOS.

Services:
- BookDB all-in-one dev process or separate roles;
- PostgreSQL single node;
- NATS single node with JetStream;
- Valkey single node;
- OpenSearch single node;
- SeaweedFS single-node/dev S3 endpoint.

This profile is not HA.

## Profile B — Standard Compose

Purpose: one production host where simplicity is more important than hardware HA.

Services are separate containers:
- 2+ API containers optional;
- scheduler;
- worker pools;
- PostgreSQL;
- PgBouncer;
- NATS JetStream;
- Valkey;
- OpenSearch;
- SeaweedFS.

Horizontal workers improve throughput but one physical host remains a failure domain.

## Profile C — HA Functional Lab

Multiple replicas of stateful services on one or more lab hosts to exercise:
- PostgreSQL promotion;
- NATS quorum;
- OpenSearch node loss;
- S3 node loss;
- API/load-balancer failover.

A single-host HA lab must be labeled “failure-testing only,” not true availability.

## Profile D — Distributed Production

Minimum conceptual layout:

```text
LB1/LB2
  |
API xN

PG/Patroni x3
etcd x3
PgBouncer x2

NATS/JetStream x3

OpenSearch x3+

SeaweedFS distributed cluster

Valkey primary/replicas + Sentinel
or Valkey Cluster

Worker pools xN
Scheduler x2+
```

Place voting/quorum nodes across distinct failure domains where possible.

## Profile E — External Services

BookDB provides only application roles; operator provides:
- PostgreSQL endpoint(s);
- NATS;
- Valkey;
- OpenSearch;
- S3 endpoint.

## Docker Compose support

BookDB ships Compose for development and standard deployment. Distributed multi-host HA cannot truthfully be guaranteed by a single-host Compose file. The project will provide topology examples and environment contracts for running the same OCI containers under multi-host orchestration or against external HA services.
