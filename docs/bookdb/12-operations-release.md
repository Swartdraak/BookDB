# Deployment, operation, recovery and release

Status: required operational deliverables. The existing Compose files are development scaffolding, not a verified production deployment. [Architecture](02-architecture.md) defines component ownership; [testing](08-testing.md) defines evidence.

## Deployment profiles

| Profile | Purpose | Required characteristics |
| --- | --- | --- |
| Development/test | Laptop or allocated test host | Disposable fixtures; loopback/internal ports; explicit dev credentials; named volumes/projects; deterministic startup/reset |
| Standard self-hosted | First supported user installation | One host, persistent volumes, reverse-proxy TLS, real secrets, backup plan, authenticated internal services and health checks |
| HA reference | Multi-host operation | Three independent failure domains for quorum services; redundant API/workers; DB failover/fencing; redundant search/object storage; measured failure behavior |

Do not prescribe hardware as if a universal catalog has a known fixed size. Initial reference allocation suggestion: 8 vCPU/32 GB RAM and sufficient SSD space for the S6 benchmark, measured and adjusted before acceptance. The owner can allocate more. Budget disk for compressed snapshots + extracted/chunked working data + PostgreSQL tables/indexes + retained claims + OpenSearch + assets + backups + migration/reindex headroom. Measure sample bytes/record and compression factors; record replication multipliers rather than guessing world-scale storage.

## Installation contract by S8

Release documentation must list supported host/architecture, tested Docker/Compose versions, exact image digests, minimum/recommended measured resources, DNS/TLS requirements and volume ownership. Provide:

1. Download verified release configuration and artifacts.
2. Create local secrets without committing or logging them.
3. Configure public URL, trusted proxy, private service addresses, storage and backup destination.
4. Start dependencies and verify protocol-level readiness.
5. Run migrations once with a migration role/lock.
6. Start API/worker/scheduler/WebUI.
7. Bootstrap local admin and optionally configure OIDC.
8. Create API key and run a sample request.
9. Start a bounded source import; confirm progress and searchable records.
10. Configure recurring backups and run a restore rehearsal.

Exact commands and actual config schema are written with the implementation. Do not ship documentation referring to a nonexistent config file or command. Production startup refuses known development secrets and unsafe public service bindings where detectable. Upgrades never delete volumes as a routine step.

## Persistence audit priorities

Verify SeaweedFS master/filer metadata persistence and the S3 credential model as well as object-volume bytes. A surviving data volume alone does not prove the bucket/index can be reconstructed. Verify PostgreSQL version-specific volume layout. Check NATS stream retention/replicas, OpenSearch security and snapshots, Valkey non-authoritative behavior, file permissions and secret injection. Existing Compose fixed container names and project name must be removed/overridden for isolated tests.

## Observability and SLOs

Expose liveness/readiness/startup and authenticated/restricted metrics. Record HTTP latency/errors/quota rejections; active/queued jobs; checkpoint lag; source freshness; accepted/rejected/quarantined records; match/merge/conflict counts; outbox age; NATS pending/redelivery; search projection lag; asset failures; DB/storage pressure. Include request/job/event/source IDs in logs, but not API keys, passwords, tokens or private proposal bodies.

Alert on overdue source sync, rising quarantine ratio, growing outbox/queue lag, failed backups, unhealthy quorum, sustained error rate and disk pressure. Thresholds derive from measured baselines; the initial runbook includes clear diagnosis and recovery for each alert. An unavailable upstream should mark freshness degraded while preserving existing catalog reads.

## Backup and restore

Back up PostgreSQL with a documented base/WAL strategy appropriate to the profile; retain source-policy revisions and publication/identity history. Back up S3 manifests and required objects plus SeaweedFS metadata according to its tested topology. Store secrets/encryption keys separately with a recovery procedure. Search is rebuildable but a tested snapshot can reduce RTO. Do not rely on Valkey for durable state.

A restore run creates a fresh isolated environment, restores the DB/object catalog, validates hashes/row counts and schema version, replays eligible outbox work, rebuilds/validates search, tests API keys/roles and runs representative catalog queries. Handle events older than restored state using durable idempotency/version checks. Measure RPO/RTO against the targets in requirements. Record date, backup identity and operator; backup completion alone is not proof of restore capability.

## HA reference and failure drills

Spread quorum members across actual hosts/failure domains: etcd/Patroni database control, NATS JetStream replication, OpenSearch cluster and an explicitly redundant S3 reference. Two API replicas share canonical DB and Valkey quota state. Define network ports, private networks, service discovery, leader endpoint and maintenance steps. Document how the former database leader is fenced to avoid split brain. A set of containers on one host is not HA.

Run host loss, network partition, worker crash, scheduler failover, storage endpoint loss, projection rebuild and rolling upgrade scenarios. For each record expected available/degraded/unavailable operations, acknowledged-write guarantees, detection time and recovery command. S6 cannot claim HA from topology diagrams alone.

## Upgrade and rollback

Use expand/backfill/validate/contract migrations when changing populated schemas. Keep old application compatibility for the declared window; migration checksums and locks prevent inconsistent application. Back up before data changes. Rollback may require restoring a backup rather than reversing SQL; state this explicitly. Test the exact supported path. Never silently change existing canonical UUIDs or erase merge history during an upgrade.

## Release lifecycle

Candidate `vX.Y.Z-rc.N` → automated acceptance → human gate → promote the same immutable artifacts to `vX.Y.Z`. Pin source commit, dependency lockfiles, container digests, generated API/client versions, fixture/snapshot IDs and migration version. Include release notes, known limitations, security changes, compatibility matrix, attribution/license notices, SBOM and checksum/signature verification instructions.

Use SemVer for API/client compatibility; explain breaking schema/config/API behavior even before 1.0. A code release does not certify global catalog completeness. Publish coverage/freshness evidence separately from software version. After release, monitor errors, ingest health, regressions and issue reports; use patch PRs and the same evidence gates.

The agent can assemble/test an RC autonomously. S8 human acceptance is required before GA promotion. Production deployment to a particular host or destructive maintenance is a separate named operational action, not implied by producing the package.
