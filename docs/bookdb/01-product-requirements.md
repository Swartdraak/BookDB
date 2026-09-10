# Product requirements

Status: target behavior. Requirement IDs remain stable across implementation changes. Stage IDs refer to [the delivery plan](07-delivery-plan.md). The milestones are release slices, not deadlines or estimates of model throughput.

## Purpose and boundaries

BookDB makes bibliographic identity durable when upstream services change, fail, disappear, or disagree. It serves independent self-hosted installations first. It is not a download manager, reader, audiobook player, ebook converter, social review platform, or personal collection manager. No central BookDB cloud service, federation protocol or model inference service is required for v1.

The aspiration is a clean, comprehensive catalog of all released books in all formats. Operationally, completeness means: all eligible records in the declared input snapshots are processed or accounted for; each supported format has a documented metadata profile; unknown/conflicting values are explicit; corrections preserve audit history. Record counts do not establish correctness. The UI must distinguish source coverage from world coverage.

The model supports print, hardcover, paperback, large print, ebook publication variants, digital and physical audiobooks, braille and other accessible editions, illustrated/graphic books, translations, revisions, abridgments, full-cast performances, anthologies, omnibuses, multi-volume sets and book-length literary works. Chapters and components are representable within a publication. Independent journal-article indexing, podcasts, news feeds, film/TV catalogs and media acquisition are outside v1.

## Personas and access

| Persona | Needs | Boundary |
| --- | --- | --- |
| Reader | Fast discovery, edition comparison, contributors, provenance and corrections | Browse published data; no direct canonical writes |
| Contributor | Propose new records and fixes with evidence; track decisions | Own drafts/proposals visible to self and administrators |
| Administrator | Review, publish, merge/split, manage sources, jobs, users, keys, quality | All actions attributable and auditable |
| Integration client | Stable IDs, search/lookup, editions/contributors, incremental updates | API key, scopes and quotas; cannot publish by default |
| Operator | Install, monitor, backup, restore, upgrade and scale | Operational access does not require editing catalog tables manually |

Public browsing is an instance setting, disabled by default for the initial self-hosted release. When enabled, the web application serves only published fields. Machine catalog access under `/api/v1` always requires an API key. The WebUI uses separate session-authenticated `/web/v1` endpoints to avoid embedding a privileged shared key in JavaScript. Public web routes remain rate-limited; API keys are an access/control mechanism, not a guarantee that visible public facts cannot be read by a browser.

## Functional traceability

| ID | Requirement and observable acceptance | Introduced / completed |
| --- | --- | --- |
| BDB-001 | Install and restart a working application with documented health checks; persisted data survives ordinary shutdown | S0 / S8 |
| BDB-002 | Stable UUID identity for works, expressions, editions, people, organizations and series; source IDs are mappings, never primary identity | S1 / S5 |
| BDB-003 | Store rich format-aware metadata and many-to-many credited roles without collapsing translations, recordings or editions | S1 / S5 |
| BDB-004 | Ingest approved open metadata snapshots in bounded memory, record checkpoints, and resume after interruption without duplicate publication | S2 / S6 |
| BDB-005 | Reconcile multiple sources with field provenance, deterministic rules, contradiction handling and human correction | S3 / S5 |
| BDB-006 | Search and browse works, contributors, series and editions, including Unicode text, facets, pagination and edition comparison | S2 / S5 |
| BDB-007 | Require API keys on catalog API routes; support scopes, revocation, rotation, expiry and request quotas | S1 / S4 |
| BDB-008 | Support local accounts and OIDC; protect session flows and enforce server-side roles | S2 / S4 |
| BDB-009 | Users submit additions/corrections; unapproved data does not reach public API, search, assets, exports or change feed | S4 |
| BDB-010 | Administrators can approve/reject with reasons; conflicts against changed canonical versions require rebase/review | S4 |
| BDB-011 | Administrators manage source policies, schedules, manual sync, pause/resume, retry, quarantine and health | S2 / S5 |
| BDB-012 | Merge and split preserve aliases, provenance, relationships, redirects and consumer resynchronization history | S3 / S7 |
| BDB-013 | Serve edition-specific covers and relevant contributor portraits only under an eligible asset policy | S5 |
| BDB-014 | Publish OpenAPI with examples and an integration guide; demonstrate Bibliophilarr search, lookup and refresh | S1 / S7 |
| BDB-015 | Durable canonical change feed and filtered bulk export support repeatable client synchronization | S3 / S7 |
| BDB-016 | Restore PostgreSQL and object data, rebuild search, safely replay events, and demonstrate multi-host failover | S6 / S8 |
| BDB-017 | Report freshness, missing fields, conflict/duplicate rates and coverage by source, language and format | S3 / S6 |
| BDB-018 | Use GitHub lifecycle tooling and update affected documents in the same change | S0 / ongoing |

## Quality acceptance

| Area | Release target selected for this project | Evidence |
| --- | --- | --- |
| Identity | 100% pass on deterministic safety fixtures; zero false merges in the hard-negative corpus | Versioned corpus and resolution report |
| Automatic matching | At least 99.5% precision on a labelled benchmark of at least 1,000 candidate pairs; report recall and counts separately | Held-out labels, confidence intervals, breakdowns; no tuning against held-out labels |
| Data accounting | Every source record is accepted, unchanged, rejected or quarantined with a reason; totals reconcile | Job summary with snapshot identity and rules version |
| API consistency | OpenAPI contract tests cover every implemented operation and security branch | CI artifact, including 401/403/409/429 behavior |
| Responsiveness | At the declared S6 benchmark: p95 lookup ≤250 ms, search ≤750 ms, error rate <1% at 20 sustained requests/s for 15 minutes | Named hardware, dataset, cache state, service versions and raw results |
| Accessibility | WCAG 2.2 AA target; automated checks plus keyboard, screen-reader and contrast review | S5/S8 human acceptance |
| Recovery | Standard profile RPO ≤24 h and RTO ≤4 h; HA profile RPO ≤5 min and RTO ≤30 min under the documented failure scenario | Timed restore/failover drills; replication alone is not backup |
| Usability | A human can complete each stage's test from its written instructions without undocumented agent state | Gate record with exact commit and environment |

These are engineering acceptance targets, not measured present performance or a universal service promise. The benchmark reference dataset is 100,000 works and 500,000 editions with realistic claims and contributors. S6 also measures a 10× dataset where resources permit, reporting actual results rather than extrapolating success. World-scale catalog completeness is never a release checkbox.

## Change control

Requirement changes need an issue explaining the user impact and updated acceptance tests. An ADR is needed only for durable architecture, identity semantics, source rights strategy or a trust boundary. A new role, document or infrastructure component is not automatically necessary for a feature. Architecture must serve these requirements, and stage scope must remain small enough to deliver demonstrable behavior.
