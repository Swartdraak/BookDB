# Domain Boundaries

## Identity vs Reconciliation

**Identity Resolution** answers:
> “Do these source/entity records represent the same real-world entity?”

**Reconciliation** answers:
> “Given claims attached to one resolved entity, which facts become canonical?”

Neither agent owns both questions.

## Source Scheduler vs Source Connector

**Scheduler** decides *when/how a connector execution is dispatched*.

**Connector** decides *how an approved source is fetched/parsed into source records and claims*.

A connector cannot install its own background polling loop.

## Database vs PostgreSQL HA

**Database** owns BookDB schema/query/persistence behavior.

**Postgres HA** owns availability/topology/replication/failover configuration.

## Backend API vs Authentication

**Backend API** owns HTTP resource contracts and handler orchestration.

**Auth Identity** owns identities, sessions, OIDC, RBAC, API-key validation and authorization semantics.

Integration points require explicit interface contracts.

## Asset Storage vs Source Connector

Connector emits source asset references/claims.

Asset Storage decides permitted retrieval/storage/transformation only after rights policy allows it.

## Platform DevEx vs DevOps Release

Platform DevEx owns local developer workflow and shared root manifests.

DevOps Release owns CI/release/container artifact production.

Shared dependency-file edits are serialized through Platform DevEx.
