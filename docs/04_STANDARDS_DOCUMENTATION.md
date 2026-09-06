# Standards Documentation

## Bibliographic/interchange

- IFLA LRM concepts
- BIBFRAME 2.0
- MARC 21
- ONIX mapping support
- Schema.org Book/Audiobook
- ISBN validation and edition semantics
- ISO language/script codes where applicable

## API

- OpenAPI 3.1
- HTTP semantics
- RFC 3339 timestamps
- `application/problem+json`
- cursor pagination
- ETag/conditional requests
- UUIDv7 internal identifiers
- explicit API versioning

## Distributed processing

- versioned NATS event schemas
- at-least-once-safe consumers
- transactional outbox
- idempotency keys
- bounded retries/dead-letter
- trace context propagation

## Persistence

- PostgreSQL 18 supported line
- migrations in source control
- expand/migrate/contract for large breaking schema changes
- PITR/WAL archive for HA production
- PgBouncer connection pooling

## Search/assets/cache

- OpenSearch as rebuildable projection
- S3-compatible object storage
- content hashes for assets
- Valkey used only for non-authoritative cache/ephemeral coordination

## Security

- OIDC/OAuth standards
- Argon2id local passwords
- secure session cookies
- least-privilege service credentials
- SBOM
- signed/checksummed releases

## UI/accessibility

- React/TypeScript
- WCAG 2.2 AA target
- semantic HTML
- keyboard navigation
- accessible admin workflows

## Versioning

- SemVer application releases
- source connector schema versions
- reconciliation algorithm versions
- event schema versions
- OpenSearch index schema versions
