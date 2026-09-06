# Product Documentation

## Product promise

BookDB provides one stable, reconciled bibliographic catalog for books, ebooks, and audiobooks, independent of the metadata source used to construct it.

## Personas

- guest
- user
- contributor
- moderator/triage reviewer
- administrator
- source manager
- operator
- API consumer
- connector developer

## Binding governance rule

Users can propose additions/corrections, but all user-provided catalog information requires **Administrator approval** before it can affect the public canonical catalog.

## Public/user capabilities

- search and browse;
- work vs edition distinction;
- ebook and audiobook details;
- series/author pages;
- alternate editions/translations;
- identifiers;
- provenance where enabled;
- propose correction/new entity.

## Administrator capabilities

- approve/reject proposals;
- merge/split;
- review evidence;
- manage series and relationships;
- approve images/rights;
- configure users/OIDC;
- configure sources/schedules;
- manually start/pause sync;
- inspect queues/quarantine;
- issue/revoke API keys;
- audit changes.

## Source-sync behavior

Internet access is normal. Sources run only on configured schedule, manual trigger, upstream event, bootstrap, or replay. BookDB does not constantly poll all providers.

## Scale

BookDB is designed for a catalog of all published works:
- horizontal workers;
- distributed search;
- durable message bus;
- S3-compatible object storage;
- HA-ready PostgreSQL;
- stateless API replicas.

## Product principles

- correctness over cosmetic completeness;
- stable BookDB IDs over provider IDs;
- provenance over opaque source priority;
- field-level authority;
- unknown remains unknown;
- user libraries are never source data;
- no user-contributed public change before admin approval.
