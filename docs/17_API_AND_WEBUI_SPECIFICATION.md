# API and WebUI Specification

## Native API

Base: `/api/v1`.

Public/catalog:
- `/search`
- `/works/{id}`
- `/expressions/{id}`
- `/editions/{id}`
- `/people/{id}`
- `/organizations/{id}`
- `/series/{id}`
- `/subjects/{id}`
- `/assets/{id}`
- `/resolve`
- `/changes`
- `/provenance/...`

User:
- `/proposals`
- `/proposals/{id}`

Administrator:
- `/admin/proposals`
- `/admin/proposals/{id}/approve`
- `/admin/proposals/{id}/reject`
- `/admin/sources`
- `/admin/sources/{id}/sync`
- `/admin/schedules`
- `/admin/jobs`
- `/admin/users`
- `/admin/api-clients`
- `/admin/audit`

## User proposal visibility

Submitted user changes do not alter public catalog responses until Administrator approval completes successfully.

## Search

Search endpoints query OpenSearch in production.
Exact canonical ID retrieval uses PostgreSQL.
Exact identifier resolution may use optimized PostgreSQL indexes/cache before search.

## WebUI

### Public/user
- search/autocomplete;
- work/edition/person/series pages;
- alternate editions/translations;
- ebook/audiobook facts;
- provenance summary;
- proposal creation/history.

### Administrator
- proposal approval queue;
- evidence comparison;
- duplicate detection;
- merge/split;
- source/schedule dashboard;
- Sync Now;
- jobs/quarantine;
- OIDC/users/RBAC;
- API keys;
- audit;
- data-quality;
- system health.

### Moderator/triage
May prepare/recommend proposals, but UI does not expose final Publish/Approve capability unless user has Administrator publication permission.

## API compatibility

Official SDKs generated from OpenAPI are Apache-2.0.
PVR-specific compatibility endpoints/adapters are isolated and independently contract-tested.
