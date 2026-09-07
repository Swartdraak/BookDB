# HA Reference Topology

This is an architecture contract, not a copy/paste production configuration.

## Minimum logical components

- reverse proxy/load balancer x2
- BookDB API x2+
- scheduler x2+
- worker pools xN
- PostgreSQL/Patroni x3
- etcd x3 or x5
- PgBouncer x2
- NATS JetStream x3 or x5
- OpenSearch x3+
- SeaweedFS distributed topology
- Valkey Sentinel or Cluster
- backup target distinct from primary failure domain

## Failure-domain rule

For actual HA, quorum replicas must not all share:
- same VM host;
- same storage array;
- same power domain;
- same network switch/path,
when the deployment goal includes surviving that failure.

## Test matrix

- API node loss
- scheduler node loss
- worker node loss
- PostgreSQL primary loss
- one etcd member loss
- one NATS member loss
- one OpenSearch node loss
- one object-storage node loss
- Valkey primary/cache loss
- reverse proxy loss
