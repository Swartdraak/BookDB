# ADR-0004 — NATS JetStream Durable Work/Event Bus

Status: Accepted

Use JetStream for durable asynchronous jobs/events. Workers assume at-least-once delivery and are idempotent. Critical streams use replication factor 3 in HA deployments.
