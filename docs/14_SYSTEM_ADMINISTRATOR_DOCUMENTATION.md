# System Administrator Documentation

## First-run

1. deploy dependencies;
2. apply database migrations;
3. create bootstrap local Administrator;
4. configure public URL/TLS/reverse proxy;
5. configure optional OIDC;
6. configure OIDC role mappings;
7. review source-policy registry;
8. configure source schedules;
9. start approved bootstrap imports;
10. monitor source/job/reconciliation/data-quality dashboards;
11. create scoped PVR API keys.

## User proposal rule

Only Administrators can approve a user-provided catalog proposal for publication.

Administrators can:
- approve;
- reject;
- request more evidence;
- redirect proposal to existing entity;
- merge duplicate proposal;
- edit approved claim before publication only with audit trail.

## Source control

Admin source page:
- enable/disable;
- schedule;
- Sync Now;
- pause;
- full/incremental mode;
- concurrency/rate budget;
- next run;
- last success;
- connector/policy version;
- quarantine.

Sources never continuously poll outside configured triggers.

## HA operations

Admin guide must document:
- PostgreSQL/Patroni health;
- JetStream quorum/stream health;
- OpenSearch health;
- S3 health;
- Valkey health;
- worker lag;
- outbox lag;
- failover drills.

## Backups

PostgreSQL/PITR and S3 assets are mandatory backup domains.
OpenSearch is rebuildable.
Valkey cache is disposable.
JetStream retention and backup policy depends on whether streams contain reconstructable vs operational-only events.

## Upgrades

Never upgrade all quorum members at once.
Run migration rehearsal and rolling-component procedures in staging first.
