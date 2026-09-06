# BookDB Master Engineering Plan — Final Baseline

## 1. Mission

BookDB will be a standalone, self-hosted, source-independent bibliographic catalog for all discoverable published book-like works, with first-class ebook and audiobook metadata. It will provide:

- a public/user WebUI;
- administrator curation and publication workflows;
- stable BookDB identifiers;
- provenance-aware canonical records;
- ingestion/reconciliation from approved external metadata sources;
- high-scale search;
- durable API integrations for PVR/library software;
- scheduled and manual source refresh;
- horizontal worker scaling;
- HA-ready persistence/search/messaging/object storage;
- cross-platform development.

BookDB does not ingest personal media libraries and does not manage user-owned copies, files, reading progress, or downloads.

## 2. Licensing decision

### Server and WebUI
**AGPL-3.0-or-later.**

Reason: BookDB is network-interactive server software and the project owner wants modified hosted versions to remain FOSS.

### Official integration SDKs and API schema
**Apache-2.0.**

Reason: maximize adoption by third-party PVR/library clients without turning ordinary API consumption into a server-license concern.

### Documentation
Recommended **CC BY-SA 4.0** for prose documentation, with code snippets following the license of the component they belong to.

### Metadata/database outputs
No single blanket license is assumed. BookDB tracks source rights at claim/asset level. Export profiles can provide:
- `open-safe`: only claims compatible with open redistribution requirements;
- `instance-full`: everything the local instance is permitted to retain/serve;
- `attribution-bundle`: exports with machine-readable attribution obligations.

BookDB-originated factual curation should be dedicated to **CC0** where legally practical, but the project must not falsely relicense third-party database rights.

## 3. Final technology stack

### Application
- Go 1.26+.
- One repository and shared domain packages.
- Independently runnable process roles:
  - API/Web process
  - scheduler
  - source-ingest workers
  - normalization workers
  - identity/reconciliation workers
  - canonical-publication workers
  - search indexers
  - asset workers
  - maintenance workers

This is a **modular distributed application**, not dozens of independent microservice repositories.

### Data plane
- PostgreSQL 18 canonical database.
- PgBouncer connection pooling.
- Patroni + PostgreSQL streaming replication + etcd reference HA topology.
- NATS JetStream durable work/event bus.
- Valkey 9 for cache, request throttling, ephemeral coordination only.
- OpenSearch 3.x for production catalog search.
- SeaweedFS as default FOSS S3-compatible reference object store.
- S3 API abstraction so Ceph RGW, cloud S3, or other compatible stores are interchangeable.

### Web
- React 19 + TypeScript.
- Vite 8.
- TanStack Query for server-state access.
- TanStack Table for large moderation/admin datasets.
- OpenAPI-generated TypeScript client.
- WCAG 2.2 AA target.

## 4. High-level topology

```text
                    EXTERNAL METADATA SOURCES
                 (scheduled/manual/event-triggered)
                              |
                       Source Scheduler
                              |
                         NATS JetStream
                              |
              +---------------+----------------+
              |               |                |
        Ingest Workers   Asset Workers   Other Workers
              |
        Raw Source Records
              |
        Normalized Claims
              |
      Identity/Reconciliation
              |
     Canonical PostgreSQL Cluster
              |
       Transactional Outbox
       /                  \
  Search Indexer       Change Publisher
      |                     |
 OpenSearch            NATS / API Change Feed
      |                     |
      +----------+----------+
                 |
              API Tier
          (N stateless replicas)
                 |
             WebUI / PVRs

Objects -> S3-compatible Object Store
Cache/rate-limit -> Valkey
```

## 5. Canonical model

BookDB uses a pragmatic LRM/BIBFRAME-inspired model:

```text
Work
  └─ Expression
      └─ Edition
          └─ MarketListing
```

Plus:
- Person
- Organization
- Contribution
- Publisher
- Imprint
- Series
- SeriesMembership
- Subject/Genre/Classification
- Identifier/ExternalIdentifier
- Award
- Asset
- Source
- SourceRecord
- Claim
- CanonicalProjection
- MergeRedirect
- AuditEvent
- ContributionProposal
- PublicationDecision

BookDB does not model an individual's owned `Item` or library copy.

## 6. Public truth model

Every canonical assertion has:
- value;
- evidence/claim IDs;
- classification: DIRECT / INHERITED / INFERRED / CURATOR;
- reconciliation algorithm version;
- confidence where applicable;
- publication state;
- effective timestamp.

Unknown remains unknown. BookDB does not synthesize unverified facts merely to increase completeness.

## 7. User contributions

User-contributed changes use a strict staging boundary:

```text
User Proposal
   -> validation
   -> duplicate/evidence checks
   -> optional moderator triage
   -> ADMINISTRATOR DECISION
       -> approve -> reconciliation/publication
       -> reject  -> retained audit record, never public
```

No user-provided field, entity, image, relationship, or identifier becomes public before Administrator approval.

Moderators may triage/recommend if enabled but cannot publish user-generated catalog changes.

## 8. Source synchronization

Connectors are dormant except when:
- a configured schedule is due;
- an Administrator uses Sync Now;
- an approved upstream webhook/event initiates a sync;
- a maintenance/recovery replay is explicitly launched.

There is no uncontrolled always-on polling loop.

Source schedules support:
- timezone;
- cron/interval policy;
- jitter;
- maintenance windows;
- per-source concurrency;
- maximum runtime;
- rate budget;
- conditional HTTP;
- full-vs-incremental mode;
- retry/backoff;
- pause state.

## 9. HA and horizontal scaling

Day-one design requirements:
- API processes are stateless and horizontally scalable.
- Workers use durable JetStream consumers.
- Jobs are idempotent and at-least-once safe.
- PostgreSQL has an HA deployment contract.
- Search runs as replicated OpenSearch cluster in distributed production.
- assets use S3-compatible replicated object storage.
- Valkey is clustered or Sentinel-managed when HA cache is required.
- no component assumes localhost-only dependencies.
- no persistent scheduler leadership is stored only in process memory.
- canonical changes are published through a transactional outbox to prevent DB/event divergence.

## 10. Search

Production search uses OpenSearch:
- title/alias multilingual search;
- people;
- series;
- publisher/imprint;
- exact ISBN/external identifiers;
- fuzzy title/author matching;
- filters for language, format, year, narrator, etc.

PostgreSQL full-text/trigram search is retained for development/minimal-recovery modes but is not the primary large-catalog search engine.

OpenSearch is always rebuildable from PostgreSQL canonical projections.

## 11. Source policy

Technical accessibility is not permission.

Each connector is gated by a versioned source policy defining:
- legal/terms references;
- acquisition modes;
- persistence permission;
- redistribution;
- attribution;
- raw retention;
- asset rights;
- rate limit;
- policy review date;
- status.

No `blocked` or `query_only_no_persistence` source may leak persistent data into canonical tables.

## 12. API

`/api/v1`

Core resources:
- search
- works
- expressions
- editions
- people
- organizations
- series
- subjects
- assets
- resolve
- changes
- provenance
- admin source/jobs/moderation endpoints

Native API is source-independent. Compatibility adapters never redefine BookDB's canonical schema.

## 13. Development portability

Required workflows work from terminal on:
- Windows
- Linux
- macOS

Supported developer interfaces:
- VS Code
- JetBrains IDEs
- GitHub Copilot CLI/Chat
- Claude Code/Desktop
- Codex CLI
- other repository-aware agent tooling

IDE configurations are convenience layers; `make`/task scripts/container commands are authoritative.

## 14. Deployment profiles

### Development
Single-host Docker Compose; reduced replicas.

### Standard
Docker Compose with separate BookDB roles, PostgreSQL, NATS, Valkey, OpenSearch, S3 object store.

### HA validation
Multiple local replicas to exercise failover semantics; explicitly not considered true hardware HA when all containers share one host.

### Distributed production
Multiple hosts/zones:
- 3+ PostgreSQL/Patroni nodes as chosen by operator;
- 3/5 etcd nodes;
- 3/5 JetStream nodes;
- 3+ OpenSearch nodes;
- replicated SeaweedFS/S3-compatible storage;
- multiple BookDB API/workers;
- redundant reverse proxies/load balancers.

### External-services
BookDB containers connect to operator-managed PostgreSQL, NATS, OpenSearch, Valkey, and S3.

## 15. Release objective

v1.0 is not “some connectors and an API.” It requires:
- full canonical identity model;
- admin-gated contribution publication;
- source policy enforcement;
- durable distributed workers;
- provenance;
- merge/split;
- search cluster support;
- HA deployment contract;
- OpenAPI;
- WebUI;
- backup/restore;
- data-quality benchmark;
- performance test;
- failure/failover test;
- release signing/SBOM;
- complete operational documentation.
