# Software Requirements Specification

## FR-100 Identity and catalog
- FR-101 BookDB SHALL issue source-independent UUIDv7 IDs.
- FR-102 merged IDs SHALL remain resolvable.
- FR-103 external identifiers SHALL be source/type scoped.
- FR-104 SHALL retain identity history.
- FR-105 SHALL support merge and split.

## FR-200 Bibliographic domain
- FR-201 SHALL model Work, Expression, Edition and MarketListing.
- FR-202 SHALL model Person, Organization and contributor roles.
- FR-203 SHALL distinguish ebook, audiobook and print publication data.
- FR-204 SHALL support translations, revisions and abridgements.
- FR-205 SHALL support series/subseries and non-integer ordering.
- FR-206 SHALL support multilingual aliases/descriptions.

## FR-300 Evidence and reconciliation
- FR-301 source records SHALL be separate from canonical records.
- FR-302 canonical facts SHALL have provenance.
- FR-303 claims SHALL classify DIRECT/INHERITED/INFERRED/CURATOR.
- FR-304 identity resolution SHALL use deterministic IDs before fuzzy matching.
- FR-305 contradiction gates SHALL block unsafe auto-merge.
- FR-306 reconciliation version SHALL be recorded.
- FR-307 unknown fields SHALL remain nullable/unknown.

## FR-400 Ingestion/scheduling
- FR-401 SHALL support bulk, REST, GraphQL, OAI-PMH, SRU/Z39.50, ONIX and static-file connector patterns.
- FR-402 imports SHALL be resumable.
- FR-403 connectors SHALL enforce source policy before persistence.
- FR-404 sync SHALL run only from configured schedule, manual trigger, upstream event, bootstrap or replay.
- FR-405 uncontrolled continuous polling SHALL NOT be permitted.
- FR-406 source schedules SHALL support concurrency/rate budgets, jitter, time zones and pause.
- FR-407 source jobs SHALL be distributed through a durable work bus.

## FR-500 User contributions
- FR-501 users MAY propose additions/corrections.
- FR-502 user proposals SHALL remain non-public until Administrator approval.
- FR-503 Moderator/triage roles MAY review but SHALL NOT publish user-provided catalog data.
- FR-504 Administrator approval SHALL be audited.
- FR-505 proposal conflicts SHALL show current canonical/source evidence before approval.

## FR-600 API
- FR-601 SHALL expose versioned REST/OpenAPI.
- FR-602 SHALL support search and identifier resolution.
- FR-603 SHALL support cursor pagination.
- FR-604 SHALL support ETag.
- FR-605 SHALL expose incremental change feed.
- FR-606 SHALL support scoped API keys and rate limits.
- FR-607 SHALL expose provenance subject to permissions.

## FR-700 WebUI
- FR-701 public search/browse.
- FR-702 work/edition/person/series pages.
- FR-703 contribution proposal workflow.
- FR-704 Administrator approval queue.
- FR-705 merge/split and evidence tools.
- FR-706 source/schedule/job administration.
- FR-707 user/RBAC/OIDC administration.
- FR-708 data-quality dashboard.

## FR-800 Authentication
- FR-801 SHALL support local accounts.
- FR-802 SHALL support standards-based OIDC.
- FR-803 SHALL support OIDC group/claim mappings.
- FR-804 SHALL support a controlled local break-glass Administrator.
- FR-805 SHALL store API keys only as secure hashes.

## FR-900 Distributed/HA
- FR-901 API SHALL be stateless/horizontally scalable.
- FR-902 worker roles SHALL scale horizontally.
- FR-903 durable jobs SHALL tolerate redelivery.
- FR-904 canonical DB SHALL support an HA deployment.
- FR-905 search SHALL support replicated multi-node deployment.
- FR-906 assets SHALL use S3-compatible object storage in production architecture.
- FR-907 event publication SHALL use a transactional outbox.
- FR-908 cache loss SHALL not corrupt canonical truth.

## Non-functional
- Cross-platform development: Windows/Linux/macOS.
- OCI production containers.
- Native Go binaries.
- WCAG 2.2 AA target.
- structured observability.
- reproducible migrations.
- backup/PITR.
- source/license governance.
- failure testing.
- scale target of tens of millions of works and >100M source/edition relationships without redesigning core identifiers or processing model.
