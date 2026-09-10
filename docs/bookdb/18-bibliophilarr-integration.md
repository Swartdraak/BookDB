# Bibliophilarr integration contract

Primary consumer: [Swartdraak/Bibliophilarr](https://github.com/Swartdraak/Bibliophilarr), inspected at `b1f36b0ee417cda0b0840b60ac505ae7ad5b7aca`. Recheck its current interfaces at S7. The inspected provider architecture offers a deliberate integration seam; it does not establish that changing a URL alone is sufficient.

## Implementation location and mapping

Proposed consumer-side package: `src/NzbDrone.Core/MetadataSource/BookDb/` with `BookDbClient`, `BookDbProvider`, DTOs and mapper, plus configuration/registry/UI/tests in the relevant existing locations. Names are proposed, not existing files. Follow Bibliophilarr's own agents/instructions and branch policy in that repository. Do not clone the application inside BookDB or import its C# implementation into the Go server.

| Inspected consumer capability | BookDB mapping | Important behavior |
| --- | --- | --- |
| `IMetadataProvider` name/priority/enabled/capability flags | Explicit `bookdb` provider registration | Advertise only implemented features; configuration includes base URL/key and timeout |
| `SearchForNewBook`, `SearchForNewEntity` | `/api/v1/search` + work/edition lookups | Keep entity kind and format distinct; do not make every hit a new book |
| `SearchForNewAuthor` | Search people with author-role filter | Narrators/translators are not automatically authors |
| `SearchByIsbn`, `SearchByAsin`, `SearchByExternalId` | `/api/v1/resolve` | Namespace-aware; unsupported/ambiguous values remain explicit |
| `GetBookInfo` | Work plus paginated editions/credits | Preserve edition-level metadata and nulls |
| `GetAuthorInfo` | Person + paginated works/role relationships | Bound requests; never hide incomplete paging as a complete bibliography |
| `GetChangedAuthors(DateTime)` | Durable cursor adapter over `/api/v1/changes` | Persist cursor; translate affected person/work events to refresh set; handle expired cursor with resync |
| Provider rate/health diagnostics | 401/429/503 and status telemetry | Retry-After/backoff, clear configuration errors, no credential logging |

Provider contract sources: [IMetadataProvider](https://github.com/Swartdraak/Bibliophilarr/blob/b1f36b0ee417cda0b0840b60ac505ae7ad5b7aca/src/NzbDrone.Core/MetadataSource/IMetadataProvider.cs), [IMetadataProviderOrchestrator](https://github.com/Swartdraak/Bibliophilarr/blob/b1f36b0ee417cda0b0840b60ac505ae7ad5b7aca/src/NzbDrone.Core/MetadataSource/IMetadataProviderOrchestrator.cs).

## Stable identity and migration

Use namespaced external IDs such as `bookdb:work:<uuid>`, `bookdb:edition:<uuid>`, `bookdb:person:<uuid>` in a consumer mapping layer appropriate to its current schema. Never strip type/namespace and reinterpret an old Goodreads/Hardcover/Open Library ID as a BookDB ID. Preserve old mappings as evidence/aliases and require a valid resolution result before attaching an existing consumer record.

A consumer dry-run reports proposed matches, ambiguities, no-matches and affected monitored/downloaded records. It must not modify library paths, downloaded-file associations or monitoring state merely because metadata provider preference changed. Back up before applying a populated consumer migration. A conflict does not justify recreating the entire consumer library.

The adapter should distinguish work and edition/performance. Do not flatten two narrations into one audiobook edition or map publisher into narrator. Existing mapper defaults/filtering must be reviewed: the inspected Open Library provider includes English/unknown edition preference in one selection path. BookDB's multilingual scope must not accidentally inherit that restriction.

## Change feed contract

Events include stable event ID, committed feed position/cursor, kind, canonical revision, affected UUIDs and redirect/split/tombstone data. Retention is documented. An expired cursor returns a clear 410/resync response; the client can fetch an eligible snapshot and resume from its high-water mark. A time-only changed-authors interface requires persisted cursor state plus an overlap/dedup strategy, not naive timestamp polling that misses equal timestamps or delayed commits.

When a merge redirects an ID, retain the old consumer mapping while resolving to the survivor. When a split is ambiguous, flag review and avoid silently reassigning downloaded books to an arbitrary successor. Test retries and interrupted refresh for idempotency.

## Public developer deliverables

By S7: complete versioned OpenAPI; generated TypeScript client; tested C# provider/client examples; curl/JetBrains HTTP examples; authentication/scopes/quota guide; cursor/pagination/resync guide; identifier and edition/contributor semantics; error catalog; changelog/deprecation policy; a minimal third-party integration tutorial. Official SDKs/API definitions use Apache-2.0; Bibliophilarr's own code retains its repository licensing. Do not copy GPL consumer code into an Apache SDK without evaluating its license requirements.

Use capability/contract tests with a pinned BookDB server and a pinned Bibliophilarr revision. Publish the tested version matrix. Readarr and Bookshelf are motivation/reference projects, not v1 compatibility promises. Additional adapters can be added later without deforming the canonical BookDB schema.
