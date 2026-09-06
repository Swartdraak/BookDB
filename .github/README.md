# BookDB Custom Agent Collection — v2

This agent kit is vendor/IDE independent and is designed to operate the full BookDB lifecycle: PM, architecture, implementation, source governance, distributed processing, HA, testing, troubleshooting, GitHub governance, releases, and maintenance.

## Core rule

Agents operate through repository documentation and GitHub workflow, not ad-hoc hidden assumptions.

## Specialist agents

- orchestrator
- product-manager
- system-architect
- bibliographic-domain
- source-governance
- source-scheduler
- ingestion-engineer
- identity-reconciliation
- database
- postgres-ha
- distributed-systems
- search
- object-storage
- backend-api
- auth-identity
- web-ui
- qa
- security
- observability-performance
- devops-release
- documentation
- incident-maintainer
- github-governor

## Required escalation

Changes involving:
- source legality -> Source Governance
- canonical semantics -> Bibliographic Domain + Architect
- merge/scoring -> Identity/Reconciliation + QA
- PostgreSQL HA -> Database + Postgres HA
- NATS/outbox -> Distributed Systems
- OpenSearch mappings -> Search
- user publication -> Auth + Domain + QA
- public API -> Backend/API
- security -> Security
- release -> DevOps/Release

## User contribution rule

No agent may implement a path where user-provided metadata becomes public without an Administrator approval event.

## Source scheduling rule

No agent may add an uncontrolled provider polling loop. All source executions must use approved schedule/manual/event/bootstrap/replay mechanisms.
