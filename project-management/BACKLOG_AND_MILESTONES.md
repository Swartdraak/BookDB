# Initial Backlog, Epics, and Milestones — Revised

## EPIC-001 Repository foundation
Monorepo, Go/React workspaces, CI, Dev Container, CLI task abstraction, AGPL/SDK licensing.

## EPIC-002 Canonical domain schema
Work/Expression/Edition/MarketListing/Person/Series/Claim/Source/Proposal/Audit.

## EPIC-003 PostgreSQL HA contract
PostgreSQL 18, migrations, PgBouncer, Patroni reference, PITR/failover lab.

## EPIC-004 NATS distributed work bus
JetStream cluster configs, subjects, consumers, retries, dead-letter, idempotency.

## EPIC-005 Source policy and scheduling
Policy registry, scheduler HA, cron/manual triggers, no uncontrolled polling.

## EPIC-006 Connector SDK
Bulk/REST/GraphQL/OAI/SRU/ONIX/static, checkpoints, quarantine, source health.

## EPIC-007 Open Library bootstrap
Bulk stream importer and changes.

## EPIC-008 Wikidata/authority identity
Wikidata + approved authority connectors.

## EPIC-009 Identity engine
Exact IDs, candidate generation, contradiction gates, gold corpus.

## EPIC-010 Reconciliation and canonical publication
Claims, field authority, inheritance, admin overrides, outbox.

## EPIC-011 OpenSearch
Index mappings, analyzers, alias rebuild, distributed cluster docs.

## EPIC-012 S3 asset subsystem
SeaweedFS reference, checksums, image rights, asset workers.

## EPIC-013 API
OpenAPI, keys, pagination, ETags, change feed, SDK generation.

## EPIC-014 Authentication/RBAC
Local auth, OIDC, group mapping, break-glass, API clients.

## EPIC-015 User proposals/Admin approval
Proposal state machine, triage, Administrator publication gate, audit.

## EPIC-016 Public WebUI
Search, canonical entity pages, provenance.

## EPIC-017 Admin WebUI
Approvals, evidence, merge/split, sources/schedules/jobs/users/health.

## EPIC-018 Additional approved sources
Europeana, DOAB, Crossref, DataCite, OpenAlex, VIAF, libraries, audiobook sources.

## EPIC-019 PVR integrations
Audiobookshelf adapter, Calibre plugin/SDK, generic client SDK.

## EPIC-020 Observability/operations
Metrics, traces, logs, SLOs, support bundles.

## EPIC-021 Distributed failure testing
PG/NATS/OpenSearch/S3/API/cache failover, backpressure and rolling upgrades.

## EPIC-022 v1 hardening/release
Load tests, migration rehearsal, backup restore, SBOM/signing, docs.

## Milestones

### M0 Repository and licenses
### M1 Canonical schema + auth skeleton
### M2 Distributed platform (PG/NATS/S3/OpenSearch)
### M3 Source policy/scheduler/connectors
### M4 Identity/reconciliation
### M5 Full bootstrap + data-quality benchmark
### M6 API/search/SDK
### M7 User/Admin WebUI
### M8 Expanded sources/assets
### M9 PVR integrations
### M10 HA/performance/security hardening
### M11 v1.0 RC/GA
