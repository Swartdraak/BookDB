# Security, Testing, and Operations

## New mandatory distributed-system tests

- NATS message redelivery/idempotency;
- worker crash after DB commit/before ACK;
- outbox duplicate publish;
- scheduler duplicate leadership;
- PostgreSQL primary failover;
- database fencing/split-brain prevention;
- OpenSearch node loss and alias rollback;
- object-storage node loss;
- Valkey loss;
- API replica loss;
- rolling deployment;
- queue backpressure.

## Auth tests

- local login;
- OIDC state/nonce/issuer/audience;
- OIDC role mapping;
- account linking;
- break-glass account;
- Administrator-only proposal publication;
- Moderator cannot publish user data;
- API-key scopes.

## Source scheduling tests

- schedule exactly-once logical dispatch under two schedulers;
- jitter;
- manual Sync Now;
- paused source;
- no background unscheduled polling;
- retry budget;
- checkpoint resume;
- conditional HTTP.

## Data quality

Gold corpora remain release gates for identity/reconciliation.

## Security

SSRF, decompression bombs, parser fuzzing, XSS, CSRF, SQL/filter injection, secrets, rate limits, dependency/SBOM, container privileges.

## Recovery

Data-quality incident:
pause affected pipeline -> preserve evidence -> scope -> fix -> benchmark -> replay canonical projection -> reindex -> verify -> postmortem.
