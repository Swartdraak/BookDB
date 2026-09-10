# BookDB — Project Handbook

Package version: 3.0.0 · Decisions recorded: 2026-09-10 · Status: implementation specification, not a claim of delivered application capability.

BookDB is a self-hosted bibliographic metadata authority for books in every publication format. It combines open metadata evidence into stable, deduplicated entities, exposes a modern reader and administrator interface, and provides a documented, API-key-protected, rate-limited API. Bibliophilarr is the first integration target.

## Read this first

| Your task | Read |
| --- | --- |
| Adopt this package into the existing repository | [Migration instructions](docs/bookdb/14-migration.md), then [repository audit](docs/bookdb/15-repository-audit.md) |
| Continue implementation | [Agent entry point](AGENTS.md), current GitHub issue, relevant stage in [delivery plan](docs/bookdb/07-delivery-plan.md) |
| Understand the product | [Requirements](docs/bookdb/01-product-requirements.md), [data model](docs/bookdb/03-data-model.md), [WebUI specification](docs/bookdb/06-webui.md) |
| Build backend or ingestion | [Architecture](docs/bookdb/02-architecture.md), [ingestion and reconciliation](docs/bookdb/04-ingestion-reconciliation.md), [API and security](docs/bookdb/05-api-security.md) |
| Test a stage | [Test strategy and executable command contract](docs/bookdb/08-testing.md), [delivery plan](docs/bookdb/07-delivery-plan.md) |
| Manage the project | [GitHub operations](docs/bookdb/09-github-operations.md), [documentation ownership](docs/bookdb/13-documentation-policy.md) |
| Configure development tools | [JetBrains and MCP setup](docs/bookdb/11-jetbrains-tooling.md), [agent operating model](docs/bookdb/10-agent-operation.md) |
| Operate or release | [Operations and release](docs/bookdb/12-operations-release.md), [risk register](docs/bookdb/16-risks.md) |
| Integrate a consumer | [Bibliophilarr contract](docs/bookdb/18-bibliophilarr-integration.md), canonical OpenAPI when implemented |
| Check factual grounds | [Research register](docs/bookdb/17-references.md) |

## Decisions that constrain implementation

1. Preserve useful BookDB code and history. Replace conflicting governance explicitly using the migration manifest; do not rebuild the repository from scratch.
2. One canonical checkout, short-lived feature/fix branches, reviewed commits, PRs to `main`. No agent-created worktrees, nested repositories, full repository copies, or per-task clones.
3. GitHub Issues own task status; Milestones own stage completion; Projects visualize the same issues. No task packets, authority leases, duplicate ledgers, or retrospective governance certification.
4. One primary agent session across GoLand, WebStorm and DataGrip. Default to sequential execution. Maximum two active children across the shared inference backend; one writer across all agents. The primary can implement when delegation is unavailable or unsuitable. No recursive delegation.
5. Development uses Ubuntu 24.04 and the user's installed JetBrains/Copilot versions. Inference is the user's network-accessible 1cat-vllm `/v1` endpoint with qwen3.8-27b-fp8 on two Tesla V100 32 GB GPUs. Do not replace the working backend or silently switch to a paid/cloud model.
6. Autonomous work includes routine issue/branch/commit/push/PR/CI and eligible merges. Stop at the explicit human acceptance gates. Never manufacture a human signoff. See the authority table in the agent operating model.
7. Every stage delivers observable application behavior and repeatable acceptance tests. Documentation is comprehensive but loaded by task; it is not a new workstream to continually expand.
8. Global, multilingual coverage is the long-term objective. A finite release cannot honestly certify every book ever released. Report coverage against named source snapshots and supported formats/languages; missing facts remain unknown.
9. Open metadata acquisition is the first-party product strategy. Goodreads is not a dependency. Consumer libraries, personal reading logs, purchased book files, or inferred user inventories are never catalog input.
10. User-proposed catalog changes remain private until Administrator approval. Approved source ingestion may automatically publish under configured source/reconciliation policies; source ambiguity goes to review.
11. Preserve the agreed Go/React/TypeScript/PostgreSQL architecture, NATS JetStream, Valkey, OpenSearch and S3 abstraction. Keep scale-out boundaries from the start; prove multi-host HA before GA. Introduce operational complexity alongside a testable capability.
12. Server/WebUI/workers/core CLI remain AGPL-3.0-or-later. Official client SDKs and API definitions remain Apache-2.0. Existing license files remain untouched by installation. Source metadata and assets retain their own rights; a code license does not relicense them.

## Authority and source of truth

The owner's current instructions supersede older project policy. This handbook and the linked active specifications supersede the retired v2 planning/governance documents. ADRs 0001–0010 remain historical decisions; [ADR 0011](adrs/0011-delivery-and-governance-reset.md) records specific amendments. Tool and platform permissions still apply.

Requirements describe intended behavior. Tests, code, release artifacts and CI establish implemented behavior. Where these differ, record a focused issue and fix the mismatch; do not alter evidence to fit the plan. Proposed commands in the testing guide must be implemented before their stage can pass.

The package was prepared against BookDB `60a4103ba3246875e6a8c05f1962278a5d19611e` and Bibliophilarr `b1f36b0ee417cda0b0840b60ac505ae7ad5b7aca`. Recheck changed paths at adoption. No repository changes, GitHub mutations, inference tests or application integration tests were performed by preparing this package.

## Day-to-day loop

Choose the next ready issue in the active milestone → inspect the relevant code and acceptance criteria → implement one bounded behavior → run meaningful tests → review the diff → commit/push/PR → merge when required checks and review policy allow → update the issue and Project → continue until a defined human gate or a real blocker.

Do not read all documents on each task. The entry point, issue, relevant specification and affected implementation normally suffice. Keep one short handoff in the issue/PR when a session ends.
