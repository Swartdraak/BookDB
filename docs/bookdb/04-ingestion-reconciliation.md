# Open metadata ingestion and reconciliation

Status: implementation policy. [Research references](17-references.md) distinguish verified source facts from BookDB design decisions.

## Source strategy

| Source | Planned role | Acquisition and conditions |
| --- | --- | --- |
| Open Library | S2 broad works/authors/editions bootstrap | Monthly bibliographic dumps; include redirects/deletes for reconciliation. Bulk API harvesting is not the plan [R06–R08] |
| Wikidata | S3 cross-source authority IDs, multilingual names and relationships | Structured data is CC0; use eligible dumps/subsets with a pinned identity [R09] |
| DOAB | S5 publisher/scholarly enrichment and additional language coverage | Metadata feeds described as CC0; validate interface and schema with a bounded sample [R10] |
| LibriVox | S5 candidate for audio-specific testable enrichment | API exists; exact metadata/asset reuse conditions must be established before enabling persistent ingestion [R11] |
| Other national libraries, publisher open feeds, authority datasets | Expand coverage after measuring gaps | Add only with documented acquisition and field/asset permissions; no blanket approval for an entire institution |
| Goodreads, proprietary subscription feeds and paid metadata APIs | Not initial dependencies | Disabled; no scraping workaround or requirement to obtain a subscription |

Open Library's licensing page does not make the blanket CC0 assertion present in the old registry. Record its actual statement and any applicable field restrictions rather than stamping every description/cover CC0. Initial import may admit the approved bibliographic factual subset while leaving assets and unclear text excluded. This is a focused source configuration decision, not a new general governance project.

No reading logs, ratings histories linked to readers, personal library exports, local EPUB/MOBI/PDF/M4B files, download queues or library scans are source inputs. Integration requests query BookDB and consume its output; they do not seed the catalog from the caller's inventory.

## One source policy record

For each enabled source, store: source ID; connector version; acquisition modes; allowed hosts; documentation/terms URLs and checked date; field inclusion/exclusion; raw-record retention scope; asset policy; redistribution/attribution requirements; trigger settings; per-host rate/concurrency limits; enabled state. Administrator changes are audited. Do not require a lease/approval workflow for every run of an unchanged approved policy.

Unknown source or unconfigured permission scope fails before acquisition. Policy is rechecked before publication/export and when it changes. Suspend only the affected source/fields; existing unaffected application work continues. Rights removal creates a retraction/tombstone/update event, removes affected search/export/asset access, and follows retention obligations for raw data and backups. Never relabel a source's license to bypass a policy failure.

## Durable pipeline

1. **Schedule or trigger.** Materialize a durable job keyed by source + snapshot/revision + operation. Run only on explicit schedule, manual request or approved upstream event. Coalesce repeated triggers; no uncontrolled polling.
2. **Acquire.** Stream to S3 or bounded temporary storage, compute a hash and record size/source/date. Use a published checksum when available; a locally computed hash proves local reproducibility, not upstream authenticity. Prevent SSRF, private-address redirects, archive bombs and unbounded responses.
3. **Parse.** Stream records; limit line/record/decompressed sizes. Normalize identifiers/text/dates without discarding originals. Record schema incompatibility as quarantine, not silent truncation.
4. **Persist evidence.** Store source record revision, extracted claims and manifest references transactionally. Every record ends in accepted/unchanged/rejected/quarantined state.
5. **Resolve.** Generate candidates by valid source crosslinks/identifiers and bounded title+contributor blocking. Apply hard contradictions before match scores. Ambiguity remains unresolved.
6. **Reconcile.** Select fields using field-specific evidence, corroboration, freshness and administrator overrides. Record reason and rules version. Missing newer source data does not automatically erase a supported older value.
7. **Publish.** Commit canonical revision, provenance, audit and outbox atomically. Reconciliation never publishes an unapproved user proposal.
8. **Project.** Update search and change feeds by canonical revision. A job is not “searchable” until projection catches up; expose both processing and indexing progress.

## Checkpoint, retry and load contract

Checkpoint only after durable chunk commit. A checkpoint includes snapshot hash/version, parser/rules version, logical record position and committed chunk ID. A gzip byte offset alone is insufficient for arbitrary restart; use resumable chunking or replay from a safe point with idempotent record keys. The same record/version cannot produce duplicate canonical events on replay.

Default source HTTP concurrency is one per host. Respect provider policy and `Retry-After`; exponential backoff with jitter and bounded attempts (initial design: five attempts before quarantine/failed-job state). A source circuit breaker pauses new network attempts after repeated failures. Input validation failures are not retried as transient errors. Unbounded catch-up jobs are prohibited. Users can pause, cancel, resume or replay a named job; cancellation means no new chunks after the active transaction safely finishes.

Open Library's published API guidance currently lists 1 request/s unidentified and 3 request/s identified and directs bulk consumers to dumps [R08]. Those are upstream limits, unrelated to BookDB client quotas. Default BookDB ingestion still uses dumps rather than using an allowed request rate to harvest a whole catalog.

## Matching policy

| Evidence | Decision |
| --- | --- |
| Same source key and record version | Idempotent update/no-op, not an independent corroborating source |
| Valid shared identifier with compatible edition/format facts | Candidate for exact resolution; conflicting assertions require review |
| Explicit trusted source work/edition link | Relationship evidence; retain source and validate target type |
| Similar title and contributor | Candidate generation only |
| Same person name | Never auto-merge solely on that basis |
| Different narration, abridgment, translation or incompatible publication variant | Prevent inappropriate expression/edition merge |
| Administrator override | Highest authority within its declared field/scope until explicitly revoked; source updates cannot silently replace it |
| Unknown or contradictory identity | Create separate provisional entity or unresolved candidate; expose review state |

Do not invent a numeric threshold and call it confidence. Use rule scores initially with recorded reasons. Calibrate automatic match thresholds against the labelled benchmark; report precision and recall separately. Minimize false merges even when that leaves more candidates for review. No LLM is needed in the production ingestion path. Future probabilistic/ML proposals must be isolated from automatic publication until validated and explicitly adopted.

## Merge and split

Merge chooses a surviving UUID, migrates relationships with conflict checks, adds a redirect from the retired UUID, and emits a revisioned change carrying affected IDs. Detect redirect cycles and bound chain resolution. Split creates explicit successor assignments for source records/claims/editions, retains the old identity event, and tells consumers when automatic reassignment is ambiguous. A blind “undo” is unsafe after later edits; use an inverse operation checked against current revisions or route to an administrator conflict screen.

Test source replay in different orders, duplicate delivery, late older records, source deletion, conflicting ISBN, two same-name authors, same-title different works, translations and two narrations. Every automated merge needs a reason that a human can inspect.

## Coverage and ongoing updates

S2 begins with a bounded, pinned real subset; S6 proves complete processing of a selected source snapshot. Keep parser fixtures small in Git and large snapshots in operator storage. Report eligible total, completed, rejected/quarantined, language distribution, format distribution, duplicate candidates, fields known/applicable, and last successful sync. “100% processed” means that snapshot was accounted for, not that the world's books are complete.

Prioritize additional sources from measured coverage gaps, especially contemporary audiobooks and underrepresented languages. A source with weak audio detail is not remedied by inferring a narrator from an author credit.
