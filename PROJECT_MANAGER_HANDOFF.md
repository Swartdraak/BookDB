# Project Manager Handoff

## Mission

Deliver BookDB as a global, self-hosted source-of-truth platform for books, ebooks and audiobooks.

## Read first

1. `README.md`
2. `docs/MASTER_ENGINEERING_PLAN.md`
3. `docs/09_REQUIREMENTS_DOCUMENTATION.md`
4. `project-management/BACKLOG_AND_MILESTONES.md`
5. `project-management/RISK_REGISTER.md`
6. `project-management/DECISION_REGISTER.md`

## Fixed decisions

- FOSS;
- server AGPL-3.0-or-later;
- user proposals require Administrator publication approval;
- local + OIDC;
- scheduled/manual Internet sync;
- Go/React/PostgreSQL/NATS/OpenSearch/S3/Valkey;
- HA/horizontal workers from day one;
- Docker Compose supported but not falsely marketed as multi-host HA.

## PM gates

Do not accept milestone completion without:
- documented acceptance evidence;
- tests;
- migration impact;
- API compatibility impact;
- data-quality impact;
- source-policy impact;
- HA/operational impact;
- docs.

## Critical path

M0 -> M1 -> M2 -> M3 -> M4 -> M5 -> M6/M7 -> M8/M9 -> M10 -> M11.
