# API, authentication and publication security

Status: target contract. `api/openapi-outline.yaml` is currently an outline. S1 promotes it into the canonical, validated OpenAPI specification without maintaining a second contradictory schema. Endpoint names below are implementation targets, not claims that they currently run.

## API conventions

Base path `/api/v1`; JSON UTF-8; UUID resource IDs; RFC3339 UTC timestamps; integer durations; explicit date precision; cursor pagination; stable ordering. Client API authentication uses `X-API-Key`. Use one convention throughout docs, generated clients and middleware. Do not put keys in URLs, logs, browser bundles or example files [R19]. All `/api/v1` catalog operations require a valid key, including search and asset access. Public minimal liveness and documentation can be outside that namespace.

| Operation | Contract |
| --- | --- |
| `GET /api/v1/search` | `q`, entity kind, language, format, contributor, series and `limit/cursor`; filters validated; bounded query length and complexity |
| `GET /api/v1/works/{id}` | Published work, relationships/summary and canonical revision; editions paginated separately |
| `GET /api/v1/works/{id}/editions` | Filterable edition list; includes expression relationships and media type |
| `GET /api/v1/expressions/{id}` | Language/version/performance data and credits |
| `GET /api/v1/editions/{id}` | Publication metadata, identifiers, contributors, contents and eligible assets |
| `GET /api/v1/people/{id}` / `/organizations/{id}` | Identity labels, public biography/relationships and eligible portraits |
| `GET /api/v1/people/{id}/works` | Stable paginated bibliography; role filtering |
| `GET /api/v1/series/{id}` | Ordered memberships and order-scheme metadata |
| `GET /api/v1/resolve` | `namespace`, `value`, optional entity kind; returns `resolved`, `ambiguous`, `not_found`, `merged` or `split` with candidate/redirect details |
| `GET /api/v1/changes` | Opaque cursor, optional entity-kind filter; durable event ID, revision, event type and affected IDs |
| `GET /api/v1/assets/{id}` | Eligible bytes or short-lived signed URL; do not expose an unrestricted S3 bucket |
| `GET /api/v1/entities/{kind}/{id}/provenance` | Public evidence and source attribution; redact private proposals, private actor details and credentials |
| `POST /api/v1/proposals` | `proposal:write` scope; private revisioned change proposal; returns 202 and proposal ID |
| `POST /api/v1/exports` | `export:create`; asynchronous eligible catalog export, bounded quotas and durable job ID |

Administrator and browser-session actions use `/web/v1`: authentication, own API keys, proposals, approval/rejection, source/job management, merge/split and account administration. They share domain services with the client API, but not an embedded administrator API key. Machine-admin scopes may be added only with a specific use case and tests; ordinary integration keys cannot approve or publish.

Default list limit 25, maximum 100; bind cursors to sort/filter and validate signatures or server state. A cursor cannot provide arbitrary SQL/filter input. Use `ETag` and conditional GET; use canonical version or `If-Match` for edits. Conflicting revisions return 409 (or documented 412 for failed `If-Match`), never silent overwrites. Reject unrecognized sort/filter fields.

Use a consistent error object: `type`, `title`, `status`, `detail`, `instance/request_id`, optional validation errors. Status meanings: 400 invalid request; 401 missing/invalid/revoked/expired key; 403 valid identity lacking scope; 404 unknown/unpublished resource without revealing private existence; 409 revision/identity conflict; 410 expired change cursor/tombstoned resource as documented; 429 quota exceeded with `Retry-After`; 503 required dependency unavailable. Cross-origin redirects must not leak keys. Merged resource lookup can return a structured redirect response with the canonical target; clients must not blindly forward credentials to an arbitrary Location.

## API keys and quotas

Generate at least 256 bits of random secret; store only a keyed hash or hash of the high-entropy secret plus a non-secret lookup prefix, owner, scopes, expiry, created/revoked timestamps and last-use metadata. Show the secret once. Comparison is constant-time after bounded lookup. Rotation creates a new key with configurable overlap; revocation invalidates caches promptly (target ≤5 seconds across replicas). Keys cannot gain scopes by changing client input. Users may create only keys within their own privileges; administrator actions remain separate.

Initial configurable quotas: ordinary key 60 requests/minute, burst 10; maximum four in-flight requests/key; expensive search/export operations have separate budgets; account aggregate quota prevents multiplying capacity by making more keys. These are BookDB defaults to tune using S6 load tests, not provider limits. Rate limits use atomic Valkey operations with bounded TTL. With Valkey unavailable, protected API routes return 503 by default rather than becoming unlimited. Counters may reset after a cache failure; the quota is operational abuse control, not billing truth. Authentication remains grounded in PostgreSQL.

Bound body sizes (initial JSON command maximum 256 KiB), result sizes, timeouts, export size/parallelism, query cost, asset dimensions and decompression ratios. Login/key-creation endpoints have distinct account/IP abuse controls. Trust forwarding headers only from configured reverse proxies. Throttling tests include two replicas sharing one key and two keys owned by one account [R20].

## Local authentication and OIDC

Local accounts: proven password-hashing library with Argon2id parameters benchmarked on deployment hardware, unique salts, generic login errors, bounded login retries, secure recovery tokens, explicit disabled-account behavior. Bootstrap admin through a one-time local command or secret-based procedure; never ship default production credentials.

OIDC: use a maintained library; authorization code flow with PKCE, state and nonce; validate issuer/audience/signature/time claims and exact callback URI; bind identities by `(issuer, subject)`, never email alone. Explicitly map approved groups to roles; default new accounts to least privilege. Account linking requires authenticated proof. Test with a disposable local OIDC provider automatically; production identity-provider sign-in is human acceptance.

Browser sessions: HttpOnly/Secure/SameSite cookies, session rotation on login/privilege change, logout/revocation and CSRF protection for state-changing requests. CORS allowlist, CSP and safe output encoding. Treat external descriptions and uploaded metadata as untrusted; sanitize allowed markup. An OIDC configuration introspection endpoint is not an authentication implementation.

## Moderation transaction

States: draft → submitted → approved/rejected/needs_changes → published (for approved revision). Superseded records remain in history. Approval binds proposal revision and expected canonical version, records administrator and reason, and runs publication atomically. If prerequisites changed, return conflict rather than approving stale content.

Anonymous users cannot submit by default. Contributors can view their own proposals; administrators can review all. Neither normal client keys nor a search worker can publish user input. An administrator may submit and approve an editorial correction with an auditable decision; requiring two different humans would be impractical for a self-hosted single-admin instance and is not required.

Privacy tests must query API, search, export, asset URLs and change feed before and after approval. A proposal referencing a private uploaded cover does not make that asset public. Audit details must not expose confidential proposal text through a public provenance endpoint.

## Operational security

Use least-privilege DB accounts for runtime, migrations and DataGrip inspection; secret references through environment/files/secret manager; TLS at trusted boundaries; controlled egress for connectors; immutable audit entries at application level with restricted database writes. Do not claim tamper-proof auditing against a database administrator. Backup keys separately and verify restore permissions. Security fixes get focused issues and tests; no general “security certification” sprint is required before a small feature can ship.
