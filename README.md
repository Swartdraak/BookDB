# BookDB — Complete Engineering & Development Package v2

BookDB is a standalone, self-hosted bibliographic metadata platform designed to become a unified source of truth for **books, ebooks, and audiobooks**. It is not a personal library manager. Readarr, Bibliophilarr, Audiobookshelf, Calibre, and other PVR/library applications are consumers of BookDB; their user libraries are never BookDB metadata sources.

This revision incorporates the project owner's confirmed decisions:

1. BookDB remains fully FOSS.
2. **AGPL-3.0-or-later** is the recommended license for BookDB server/WebUI/worker code.
3. Official client SDKs and OpenAPI-derived integration libraries use **Apache-2.0** to maximize third-party integration.
4. Users may propose additions/corrections, but **all user-provided catalog changes require Administrator approval before publication**.
5. Local authentication and generic **OIDC** federation are both supported.
6. Internet-connected metadata synchronization is normal, but connectors run only on explicit schedules, manual triggers, or upstream event triggers — never uncontrolled constant polling.
7. Technology stack is selected by suitability rather than a preselected language.
8. Horizontal workers, durable distributed messaging, S3-compatible object storage, distributed search, and HA database topology are architectural requirements from day one.
9. Docker Compose remains a supported deployment and development path; true multi-host HA is a separate distributed deployment profile using the same OCI containers.
10. Development is IDE- and OS-independent.

Current branch status: M0 repository foundation. The accepted ADRs already cover the architecture decisions summarized below, so this refresh is documentation alignment only; no new ADR was required.

## M0 foundation at a glance

- Repository layout currently centers on `compose.yaml`, `deployment/compose/`, `config/`, `internal/config/`, `docs/`, `adrs/`, `.github/`, and `project-management/`.
- The runtime trees `cmd/`, `web/`, `migrations/`, and `schemas/` are reserved by the architecture docs but are not yet part of this branch.
- Developer setup starts with Go 1.26+, Docker Compose, `.env.example`, and the root development compose file.
- Compose support is dev-only: `compose.yaml` starts the core services, and `deployment/compose/app.dev.yaml` layers in BookDB app services once a BookDB image exists.
- The local app image can be built with `make compose-app-build`, then started with `make compose-app-up` or the equivalent `docker compose -f compose.yaml -f deployment/compose/app.dev.yaml up -d --wait`.
- Configuration is typed in `internal/config/` with precedence of defaults < YAML file < environment variables < secret/runtime references.
- Current executable validation is the config package test surface; the broader CI matrix in docs is target-state until the product workflows land.
- Canonical agent-governance docs live under `.github/README.md` and `.github/AGENT_ORCHESTRATION_MATRIX.md`.

## Final recommended stack

| Layer | Selection |
|---|---|
| Server language | Go 1.26+ |
| API | Go `net/http` ecosystem + OpenAPI 3.1 contract |
| WebUI | React 19 + TypeScript + Vite 8 |
| UI data | TanStack Query + TanStack Table |
| Canonical DB | PostgreSQL 18 |
| DB pool | PgBouncer |
| DB HA | PostgreSQL streaming replication + Patroni; etcd DCS in reference HA topology |
| Durable work/event bus | NATS + JetStream |
| Cache/rate-limit/short-lived locks | Valkey 9 |
| Search | OpenSearch 3.x |
| Object storage | S3-compatible abstraction; SeaweedFS is the default FOSS reference deployment |
| Observability | OpenTelemetry + Prometheus/OpenMetrics + structured JSON logs |
| Containers | OCI / Docker |
| Local orchestration | Docker Compose |
| HA orchestration | container-platform-neutral; reference topology supports multi-host Kubernetes/Nomad/Swarm-style scheduling or externally managed services |
| Native distribution | Go binaries for Linux, Windows, macOS |
| JS distribution | `@bookdb/sdk`, `@bookdb/cli`; server is not an npm application |

## Why AGPL-3.0-or-later

BookDB is primarily network-server software. AGPL is specifically intended to ensure that modified versions made available over a network also make their corresponding source available to those users. This aligns with the owner's goal that BookDB remain genuinely FOSS even if hosted or modified by third parties.

The public API definition and client SDKs are deliberately Apache-2.0 so BookDB can be integrated into both FOSS and proprietary client applications without forcing the server's copyleft model onto ordinary API consumers.

## Primary design principle

```text
Upstream metadata is evidence.
BookDB is the identity and reconciliation authority.
```

Source connectors do not write canonical rows. They create source records and claims. BookDB resolves identities, reconciles evidence, records provenance, and publishes its own stable canonical entities.

## Package navigation

Start here:

1. `docs/MASTER_ENGINEERING_PLAN.md`
2. `docs/09_REQUIREMENTS_DOCUMENTATION.md`
3. `docs/10_ARCHITECTURE_AND_DESIGN_DOCUMENTATION.md`
4. `docs/16_DATA_MODEL_AND_RECONCILIATION.md`
5. `docs/19_FINAL_TECHNOLOGY_STACK.md`
6. `docs/20_DISTRIBUTED_PROCESSING_AND_HA.md`
7. `docs/21_SYNCHRONIZATION_SCHEDULING.md`
8. `docs/22_MODERATION_AND_PUBLICATION_WORKFLOW.md`
9. `docs/23_AUTHENTICATION_AND_IDENTITY.md`
10. `docs/15_SOURCE_POLICY_AND_RESEARCH.md`
11. `project-management/BACKLOG_AND_MILESTONES.md`
12. `.github/README.md`
13. `.github/AGENT_ORCHESTRATION_MATRIX.md`

`MANIFEST.md` lists every file in this package.

For repository setup and day-1 commands, see [docs/28_REPOSITORY_OPERATIONS.md](docs/28_REPOSITORY_OPERATIONS.md).
