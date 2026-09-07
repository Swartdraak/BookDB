# Security, Testing, and Operations

## Current M0 test surface

This branch currently has a narrow executable test surface:

- `go test ./internal/config` covers defaults, environment overrides, validation, and DSN redaction for the typed config package.
- `.github/workflows/agent-governance.example.yml` validates the agent-kit governance files, not the product runtime.

The broader distributed-system matrix below is the target state for later phases.

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

## CI summary

On the M0 foundation branch, product CI is still being assembled. The only committed workflow today is the agent-governance example, so the required checks listed in [27_GITHUB_REPOSITORY_BLUEPRINT.md](27_GITHUB_REPOSITORY_BLUEPRINT.md) remain the canonical release gate rather than an already-available end-to-end pipeline.
