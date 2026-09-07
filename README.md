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

Current branch status: M1 canonical schema and authentication skeleton baseline. The accepted ADRs already cover the architecture decisions summarized below; M1 implementation adds foundational migration and auth runtime surfaces consistent with those decisions.

## M1 baseline at a glance

- Canonical schema migrations now include a foundation for `work`, `expression`, `edition`, `market listing`, and governance evidence tables under `internal/database/migrations/`.
- Runtime now exposes auth skeleton introspection endpoints (`/auth/mode`, `/auth/oidc`) without exposing secrets.
- Config validation now enforces M1 auth guardrails: at least one auth mode enabled; OIDC requires issuer, client id, and client secret or secret reference.
- Compose support remains development-oriented; distributed HA remains a later milestone concern.

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
