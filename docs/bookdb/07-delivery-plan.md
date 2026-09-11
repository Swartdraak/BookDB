# Staged delivery and acceptance plan

Status: planned work. Existing M0/M1 are historical milestones; these **S0–S8** stages replace the old M2–M11 sequencing without pretending prior work is absent. Never reopen a completed historical milestone merely to rename it.

Work through small issue-linked PRs within each stage. Every PR changes testable behavior or directly supports that behavior. S0 is the only bounded adoption stage; no continuing governance-certification phase is authorized. After S0, spend at most a small part of a delivery cycle updating affected documentation/configuration; do not treat a percentage as a reason to skip necessary security fixes.

`python3 scripts/project/acceptance.py S2 --describe` prints a stage's acceptance contract. `python3 scripts/project/acceptance.py S2` runs its implemented `scripts/acceptance/s2.sh`. Missing runners fail as **NOT IMPLEMENTED**. The package supplies the dispatcher and contracts; agents must implement real runners with the corresponding application changes. Never add no-op runners to make a stage green.

The human gates are S5, S7 and S8. S2 human review is deferred to S5 and is not an independent stop. Other stages still have human-runnable instructions, but their completion can be established by automated evidence. At a gate, stage automation and review finish first; prepare a working environment and one consolidated test handoff, then stop for the owner. Human gate issues remain open until actual human results are recorded. If the owner finds a defect, reopen the affected implementation issue, fix and retest, and resubmit only the relevant human steps plus necessary regression checks.

If an unrelated credential/admin capability blocks GitHub Projects/Wiki setup, log one capability issue and continue the ready application work. If a mandatory application or human acceptance criterion is blocked, the stage is not complete. Do not invent reasons to request human testing for routine unit/integration work.

## Stage map

| Stage | Outcome | Human gate |
| --- | --- | --- |
| S0 | Adopt the package and run the preserved application | Automated completion |
| S1 | Key-protected canonical catalog API | Automated completion |
| S2 | First real-source catalog in the WebUI | Deferred to S5 |
| S3 | Cross-source reconciliation and durable identity | Automated completion |
| S4 | Accounts, moderation and administrator controls | Automated completion |
| S5 | Rich multilingual and multi-format catalog experience | Yes |
| S6 | Bulk operation, recovery and multi-host HA | Automated completion |
| S7 | Bibliophilarr integration and public developer contract | Yes |
| S8 | Release candidate and sustainable maintenance | Yes |

## S0 — Adopt the package and run the preserved application

**Prerequisite:** Reviewed repository baseline and current owner authorization. **Requirements:** BDB-001, BDB-018.

**Deliver:** Apply the exact governance replacement, wire the missing cmd/bookdb entry point and real database/migration connection, make ordinary shutdown preserve volumes, and build/run the application image. Preserve existing Go/Web tests and working adapters. Establish GitHub Issues/Milestones/Project and essential CI without waiting for optional Wiki or account permissions.

**Dataset/environment:** Existing migrations in an empty disposable PostgreSQL database; no real catalog needed.

**Automated acceptance:**

1. Run existing Go/Web checks and the actual application Docker build; go build ./... alone is insufficient.
2. Start the app with documented dev configuration; GET /health/live returns 200, database-backed readiness returns 200 only after a successful query, and a stopped database causes readiness 503 while liveness stays 200.
3. Run migrations twice; the second run is a no-op. Insert a disposable sentinel row, stop/restart using the ordinary command, and prove the sentinel remains.
4. Check git worktree list: only the canonical checkout; no active legacy agents/leases/validator references. Verify one issue-linked PR contains adoption and the observable runtime repair.

**Human procedure:** Follow the fresh-install commands, open the health URL, inspect the sentinel in DataGrip, restart and confirm it remains. This procedure is human-runnable but does not require a human gate.

**Exit condition:** An executable application and honest baseline report exist. Any pre-existing failures are reproduced and tracked; S0 cannot pass with a missing runtime entry point.

**Initial issue slices:**

- `S0-ADOPT-RUNTIME` — **Replace legacy governance and prove a runnable preserved baseline.** Implement the migration runbook and S0 runtime tests in one bounded delivery PR.
- `S0-GITHUB` — **Configure native GitHub lifecycle tooling.** Apply idempotent labels/milestones/issues setup, set the Project fields/views, establish required checks after successful runs and record any account-level capability blocker.

## S1 — Key-protected canonical catalog API

**Prerequisite:** S0. **Requirements:** BDB-002, BDB-003, BDB-007, BDB-014.

**Deliver:** Extend the existing schema through forward migrations; implement real catalog persistence, initial work/edition/person APIs, namespace-aware resolve, CLI key bootstrap and shared Valkey quotas. Expand the OpenAPI outline to the implemented operations. This is the first machine-consumable catalog slice.

**Dataset/environment:** Small synthetic fixture catalog with fixed UUIDs: print/ebook/audio variants, two same-name people, translation, missing ISBN and conflicting identifier claims.

**Automated acceptance:**

1. GET catalog operations without a key returns 401; valid read key returns 200; invalid/expired/revoked keys return 401 and insufficient scope returns 403.
2. Resolve fixture identifiers and UUIDs; ambiguous identifiers return explicit candidates without merging records. Unknown IDs return 404.
3. Load fixtures twice without changing entity IDs/counts. Upgrade a populated baseline database and verify retained rows/relationships.
4. Exceed a small test quota and assert 429 plus Retry-After. Alternate requests between two API replicas and verify the shared limit. Stop Valkey and assert documented 503.
5. Validate response bodies against OpenAPI and assert key values never appear in response metadata or captured logs.

**Human procedure:** Use a terminal HTTP client or JetBrains HTTP file with a locally stored key, inspect a work and its editions, revoke the key, and confirm the request fails. Inspect relationships in DataGrip.

**Exit condition:** Protected API reads, migration safety, identifier ambiguity and rate-limit behavior are demonstrated with real services.

**Initial issue slices:**

- `S1-CATALOG` — **Evolve schema and implement fixture-backed canonical reads.** Preserve baseline data and implement typed relationships/identifier assertions required by S1.
- `S1-KEYS` — **Implement API keys and shared quotas.** Deliver key creation/revocation and middleware with two-replica quota tests.
- `S1-CONTRACT` — **Publish executable API contract examples.** Contract-test all S1 operations and add repeatable human HTTP examples.

## S2 — First real-source catalog in the WebUI

**Prerequisite:** S1. **Requirements:** BDB-004, BDB-006, BDB-011.

**Deliver:** Implement the approved bibliographic source policy, bounded streaming importer, S3 manifest storage, NATS-backed jobs/outbox projection, manual trigger and monthly snapshot schedule. Add minimal local bootstrap-admin login and secure browser sessions for the first private WebUI; full account management and OIDC follow in S4. Make actual imported records searchable in OpenSearch and browsable through a basic polished catalog UI. Use one-time local administration commands until S4 admin screens exist; never expose unauthenticated admin mutations.

**Dataset/environment:** Pinned subset of an Open Library bibliographic dump: at least 1,000 works with associated editions/authors, plus deliberately malformed local parser fixtures. Keep source snapshot hash and selection method.

**Automated acceptance:**

1. Run source import to completion; accepted + unchanged + rejected + quarantined equals records examined. Reimport produces no duplicate entities or duplicate unchanged publication events.
2. Interrupt after a committed chunk; resume and compare final IDs/counts with an uninterrupted run.
3. Find a known imported title and author via API and UI, open an edition and follow provenance to its source record. No mock result is acceptable.
4. Take the upstream source offline; existing catalog search and lookup still work. Pause/resume a job and show checkpoint/progress.
5. Test search loading/empty/error states, Unicode input and keyboard navigation. Verify the scheduled occurrence key prevents duplicate jobs.

**Human procedure:** Fresh-install or upgrade using the runbook, launch the bounded source job, find five named records from its manifest in the WebUI, inspect an edition and source provenance, then test the API key path.

**Exit condition:** The first real-data product is usable. Prepare the exact commit, URLs, credentials setup, fixture list, expected results and reset instructions. S2 human review is deferred to S5; proceed to S3 after automated acceptance passes.

**Initial issue slices:**

- `S2-IMPORT` — **Deliver resumable Open Library snapshot ingestion.** Use the approved field subset, durable job state, source manifests and idempotency.
- `S2-SEARCH` — **Project canonical catalog into search.** Deliver transactional outbox, NATS redelivery handling and OpenSearch reads.
- `S2-BROWSE` — **Deliver real-data search and detail UI.** Implement minimal local-admin session access plus search/results/work/edition navigation with honest empty/error states.
- `S2-ACCEPT` — **Human acceptance of the first real-data catalog (deferred to S5).** Run the S2 human procedure during the consolidated S5 review; an AI agent may prepare evidence but cannot close this gate. Linked to S5-ACCEPT.

## S3 — Cross-source reconciliation and durable identity

**Prerequisite:** S2. **Requirements:** BDB-005, BDB-012, BDB-015, BDB-017.

**Deliver:** Add a second independent source, field-level selection/provenance, duplicate candidate review data, canonical revisions, merge/split operations and change feed. Cross-source ingestion must improve evidence while preserving distinct publications and contributor identities.

**Dataset/environment:** Pinned Wikidata subset linked to S2 plus a labelled identity corpus of at least 1,000 pairs, including hard negatives and source disagreement.

**Automated acceptance:**

1. Run held-out identity benchmark: automatic-match precision ≥99.5%, zero false merges on hard negatives; publish counts, recall and uncertainty.
2. Replay sources in both orders and replay old revisions; assert the same expected canonical outcomes and no overwrite by older evidence.
3. Merge two fixture duplicates, resolve the old ID, then perform a version-checked split. Assert no redirect cycles and all affected relationships/events survive.
4. Simulate crash after DB commit before NATS acknowledgement; redelivery creates no duplicate side effects. Rebuild a projection from durable state.
5. Consume changes from a cursor; disconnect/reconnect; assert monotonic committed publication order and no skipped events under concurrent transactions.

**Human procedure:** Inspect conflicting claims for a named fixture, compare the selected value with its evidence, run a merge/split in the disposable catalog and query both old and new IDs.

**Exit condition:** Identity operations and provenance are explainable and reproducible, with durable client-visible changes.

**Initial issue slices:**

- `S3-CORRELATE` — **Add Wikidata evidence and deterministic reconciliation.** Implement field rules, hard contradiction gates and a labelled benchmark.
- `S3-IDENTITY` — **Implement revision-safe merge and split.** Preserve redirects, source assignments and typed relationships.
- `S3-CHANGES` — **Deliver committed-order change feed and replay.** Ensure change cursors cannot skip a transaction that commits late; implement expiry/resync behavior.

## S4 — Accounts, moderation and administrator controls

**Prerequisite:** S3. **Requirements:** BDB-007, BDB-008, BDB-009, BDB-010, BDB-011.

**Deliver:** Implement local authentication, OIDC, session security, RBAC, own-key management and admin source/job controls. Deliver proposal submission, comparison, revision-bound approval/rejection and publication. Public source ingestion and private user proposals follow separate eligibility rules.

**Dataset/environment:** Disposable local OIDC provider; admin, contributor, reader and API client accounts; proposals including private text and a private cover.

**Automated acceptance:**

1. Test local login/logout/reset/disabled user and OIDC issuer/audience/state/nonce/PKCE/account-linking negative cases.
2. A contributor submits a correction; it is absent from public API/search/export/asset access/change feed until Administrator approval.
3. Non-admin approval attempt returns 403; stale canonical or proposal revision returns 409/412 with no partial publication.
4. Approval creates one published revision/event and visible approved change. Rejection leaves the public record unchanged. Repeated approval is idempotent or returns a documented conflict.
5. Rotate/revoke keys across replicas; test CSRF, session fixation prevention, direct-object access, script content and input limits.

**Human procedure:** Sign in as contributor and admin in separate browser contexts, submit a change, verify privacy, reject one proposal and approve another. Use production OIDC only in the later human acceptance environment.

**Exit condition:** Security and moderation workflows pass real integration/E2E tests; human usability/OIDC acceptance is consolidated into S5.

**Initial issue slices:**

- `S4-AUTH` — **Complete local/OIDC authentication and session roles.** Replace skeleton-only auth with real flows, bootstrap and account/key UI.
- `S4-MODERATE` — **Implement private proposal review and atomic publication.** Cover private assets, stale versions, role boundaries and audit.
- `S4-ADMIN` — **Deliver source/job administration.** Expose schedules, trigger/pause/resume/quarantine and clear permission boundaries.

## S5 — Rich multilingual and multi-format catalog experience

**Prerequisite:** S4. **Requirements:** BDB-003, BDB-005, BDB-006, BDB-013, BDB-017.

**Deliver:** Complete contributor roles, organizations/imprints, series orders, aggregate contents, audio performance fields, eligible covers/portraits, edition comparison and source/quality dashboards. Add DOAB and a verified eligible audio connector where useful. Complete responsive design and accessibility. A source access blocker does not justify fabricated audio metadata.

**Dataset/environment:** Curated multilingual fixture set with Latin, Hangul, CJK and RTL text; print/ebook/braille/large-print/audio, alternate narrations, abridgment, anthology, omnibus and organization credits. Add one eligible real open audio source or explicitly keep audio coverage gate blocked.

**Automated acceptance:**

1. Assert no loss of Unicode, date precision, unknown values, series positions or contributor role/order through ingest→DB→API→UI.
2. Two narrations and an abridgment remain distinct; multi-expression edition contents and publisher/producer/narrator roles round-trip.
3. Eligible assets render; disallowed/private assets are absent; asset fetcher rejects private-network redirects and oversized payloads.
4. Run E2E journeys at 360/768/1440 px, keyboard checks and automated accessibility scans; capture real screenshots.
5. Report known/applicable field completeness by language/format/source rather than one misleading global score.

**Human procedure:** Run all six WebUI journeys, edition comparisons, keyboard/screen-reader and 200% zoom checks, verify language rendering and OIDC with the actual configured provider. Inspect at least ten diverse catalog records for bibliographic correctness.

**Exit condition:** A human accepts the complete catalog/moderation experience and real multi-format coverage. Stop here until the gate is explicitly passed.

**Initial issue slices:**

- `S5-FORMATS` — **Complete rich format and contributor model.** Add audio/aggregate/accessibility fields and multilingual fixtures using forward migrations.
- `S5-ASSETS` — **Add eligible assets and expanded open sources.** Verify source-specific permissions, implement bounded asset handling and real audio evidence.
- `S5-UX` — **Complete discovery comparison and administration UX.** Meet the full WebUI journeys, responsive/accessibility and quality reporting contract.
- `S5-ACCEPT` — **Human acceptance of catalog quality and WebUI.** Record actual signoff for S5 journeys, real OIDC and representative metadata.

## S6 — Bulk operation, recovery and multi-host HA

**Prerequisite:** S5. **Requirements:** BDB-004, BDB-016, BDB-017.

**Deliver:** Demonstrate full snapshot accounting, bounded worker memory and horizontal workers, monitored queues and quotas, backup/restore, search rebuild, S3 persistence, database failover and documented deployment profiles. Keep installation usable on one host.

**Dataset/environment:** Complete selected source snapshot; 100,000-work/500,000-edition reference benchmark and 10× run where resources permit; three independent HA failure domains.

**Automated acceptance:**

1. Process the full selected snapshot with pause/restart/backpressure; measure throughput/memory/disk and account for every record.
2. Run reference API load for 15 minutes at 20 requests/s; measure p95 lookup ≤250 ms and search ≤750 ms, errors <1%, with cache state and resources recorded.
3. Restore DB and object manifests/bytes into a clean environment, compare hashes/counts, replay outbox and rebuild search; meet stated RPO/RTO.
4. Kill/partition one failure domain and test PG leader transition, NATS quorum, search replicas and S3 access; no acknowledged canonical data silently lost within declared RPO.
5. Kill scheduler/worker mid-chunk, duplicate events and fill a queue; verify bounded retry, backpressure and recovery.

**Human procedure:** An operator can reproduce install/restore/failover from the runbook. Destructive tests target disposable resources only; human deployment validation is consolidated into S8.

**Exit condition:** A dated operational report establishes capacity and recovery on named hardware, with all unachieved targets visible as blockers.

**Initial issue slices:**

- `S6-SCALE` — **Prove bulk ingestion and API capacity.** Benchmark reference and expanded datasets with bounded resources and truthful reports.
- `S6-RECOVER` — **Implement verified backup restore and projection rebuild.** Restore into a fresh environment and verify source/catalog/asset integrity.
- `S6-HA` — **Demonstrate multi-host reference topology.** Test failure domains/quorum and document measured RPO/RTO.

## S7 — Bibliophilarr integration and public developer contract

**Prerequisite:** S6. **Requirements:** BDB-012, BDB-014, BDB-015.

**Deliver:** Implement the BookDB provider in a separately authorized Bibliophilarr branch/PR, using its existing interfaces. Publish complete OpenAPI, generated TypeScript client and a tested C# client/provider contract, HTTP examples and integration guide. Support scoped keys, quotas, change feed recovery and identifier migration.

**Dataset/environment:** Pinned Bibliophilarr revision and BookDB fixture catalog including ebook/audio, merge/split and missing values; existing Bibliophilarr data preserved.

**Automated acceptance:**

1. Configure a BookDB base URL/key, search books/authors, resolve ISBN/BookDB UUID and refresh author editions through Bibliophilarr.
2. Preserve book/edition/person namespaced IDs across repeated refresh; do not rewrite existing provider IDs as BookDB UUIDs without evidence.
3. Test 401/revocation, 429/backoff, BookDB outage, pagination, conditional GET, change cursor expiry, merge/split and null metadata.
4. Run a staging migration/dry-run against a backup of representative Bibliophilarr data and verify monitored/downloaded state remains unchanged.
5. Validate all implemented OpenAPI operations and ensure examples work without undocumented server behavior.

**Human procedure:** Configure the provider in a disposable Bibliophilarr instance, search/add/refresh one ebook and one audiobook, inspect edition/narrator details, revoke/rotate the key and verify recovery. Approve the integration based on actual use.

**Exit condition:** Human accepts the working integration; BookDB does not claim URL-only compatibility with Readarr/Bookshelf unless independently demonstrated.

**Initial issue slices:**

- `S7-PROVIDER` — **Implement Bibliophilarr BookDB provider contract.** Coordinate the external repository change under its own governance and branch lifecycle.
- `S7-SDK` — **Publish tested API docs and client examples.** Generate clients, add paging/quota/change-feed examples and compatibility version matrix.
- `S7-ACCEPT` — **Human acceptance of Bibliophilarr integration.** Record the tested revisions of both applications and explicit human result.

## S8 — Release candidate and sustainable maintenance

**Prerequisite:** S7. **Requirements:** BDB-001, BDB-016, BDB-018.

**Deliver:** Produce an immutable RC with release notes, known limits, migrations, SBOM, signatures/checksums, source/asset attributions, backup/upgrade instructions and supported configuration. Complete the human install/upgrade/integration acceptance before promoting that tested candidate to GA.

**Dataset/environment:** Clean install, populated previous-stage upgrade and restore environments; exact candidate image digests and source snapshot manifests.

**Automated acceptance:**

1. Run all stage automated acceptance suites and the regression/security/contract suite against the candidate.
2. Build the actual runtime image and verify pinned dependencies/images; scan the application image, not only the devcontainer.
3. Fresh install, upgrade populated data, rotate secrets, restart and restore using the released instructions; verify no default production credentials or public private-service ports.
4. Verify artifact hashes/digests, provenance/SBOM, license notices, Wiki/docs release links and GitHub issue/milestone/release consistency.
5. Ensure no open critical correctness/security/recovery blockers and no unapproved human gate; known non-blocking limits are published.

**Human procedure:** Perform fresh installation and upgrade using only release documentation, complete catalog/moderation/API/Bibliophilarr checks, and verify a restore. The owner explicitly accepts the tested RC for GA.

**Exit condition:** After human acceptance, the agent may publish/promote the exact tested candidate and close the milestone; deploying to unrelated/production hosts is not implied.

**Initial issue slices:**

- `S8-RC` — **Assemble and verify immutable release candidate.** Run release acceptance and produce complete operator/developer artifacts.
- `S8-ACCEPT` — **Human release acceptance for GA.** Record actual install/upgrade/restore/integration results and approval of the exact RC.
- `S8-GA` — **Promote accepted candidate and start maintenance.** Publish the accepted artifacts, close the milestone and triage the next patch backlog.

## Sprint and maintenance cadence

A sprint is a bounded delivery iteration inside the active stage, normally one coherent slice at a time for this inference budget. Do not create artificial one-week deadlines without throughput data. Plan the next slice, implement/review/test, merge, then reassess. Keep one active implementation issue, one pending review and at most two ready follow-ups refined in detail. Future milestone issues can remain coarse until they approach execution.

After GA, use patch milestones for defects/security, minor milestones for compatible capabilities and major milestones for deliberate breaking contracts. Continue the same tests, issue/PR traceability, source freshness work and restore rehearsals. Review stale source coverage and dependencies regularly; do not auto-close unresolved metadata defects just because they are old.
