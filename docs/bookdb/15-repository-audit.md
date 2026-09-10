# Repository audit and preservation plan

Inspected 2026-09-10 through GitHub read APIs. BookDB branch `main` resolved to `60a4103ba3246875e6a8c05f1962278a5d19611e`. This is a source inspection, not a local build/security audit or proof of production behavior. The complete tree contained 406 tracked files and was not truncated.

## Evidence and implications

| Finding | Evidence | Required response |
| --- | --- | --- |
| Worktree proliferation was real | [PR #8](https://github.com/Swartdraak/BookDB/pull/8) reports 16 nested worktrees removed and rules corrected | Retire automatic worktree use entirely in the new operating model |
| Application work exists | [PR #13](https://github.com/Swartdraak/BookDB/pull/13), migrations/auth/runtime/adapter/WebUI paths | Preserve and extend; do not restart the project |
| Latest commit is governance state | [Baseline commit](https://github.com/Swartdraak/BookDB/commit/60a4103ba3246875e6a8c05f1962278a5d19611e) certifies M1 and starts M2 planning | Treat M0/M1 as history; no more certification artifacts |
| Primary agent is forbidden to implement | `agent-kit/constitution/AGENT_CONSTITUTION.md` and `.github/copilot-instructions.md` | Replace with optional delegation and sequential implementation fallback |
| PRs require active leases and exact retrospective handoff coverage | `.github/workflows/agent-governance.yml` | Remove that workflow and its obsolete tools; preserve meaningful application/security checks |
| Runtime image targets a missing entry point | `Dockerfile` builds `./cmd/bookdb`; no `cmd/` in inspected complete tree | S0 restores/wires entry point and actually builds/runs image |
| Expected default configuration not tracked | Runtime defaults to `config/bookdb.yaml`; tree includes source registry files but no such file | S0 supply/test supported config defaults/example/generation; check whether user's local ignored file exists |
| Package build can conceal runtime gap | `Makefile` builds `go list ./...` packages; CI uses that target | Add explicit binary/application image build check |
| Frontend prose differs from manifests | README Go/React stack table versus `web/package.json` React 18.3.1 and Vite 6 range | Preserve current working versions during adoption; correct documentation and review supported release pins separately |
| Ordinary Compose stop deletes volumes | `make compose-down` uses `down -v --remove-orphans` | Change default to preserve data; explicit disposable reset separately |
| Authentication is skeletal | `internal/auth/skeleton.go` validates config and exposes non-secret mode data | Do not call it login/session/OIDC/RBAC/API-key implementation |
| Canonical schema is minimal | `0002_canonical_schema_foundation.sql` has title/language expression uniqueness, unique ISBN, no rich credit/organization relationships | Forward migrations and identity tests per data-model spec |
| Readiness uses TCP probes | `internal/runtime/runtime.go` dependency probe | Replace required dependency checks with protocol-level validation and relevant profiles |
| Metadata policy overstates one license | `config/source-registry.yaml` labels Open Library CC0 | Reconcile with actual source statement; do not remove Open Library bootstrap because optional fields/assets are ineligible |
| Current security image scan focuses on devcontainer | `security-supply-chain.yml` build target | Retain scans, add actual runtime-image security/release evidence |
| Main protection absent in returned branch metadata | `/branches/main` reported `protected: false` | Recheck live settings and configure native protections; no claim of a permanent current state |

## Preserve without a migration rewrite

Keep `internal/**`, `web/**`, `go.mod`, `go.sum`, `api/**`, `config/**`, `deployment/compose/**`, root `compose.yaml`, `Dockerfile`, `.devcontainer/**`, application scripts/tests, all existing license files and ADRs. Installer does not modify these application/config files. S0 and later issue PRs repair them deliberately with tests. Existing NATS/S3/OpenSearch clients and health/config/database tests are useful foundations even though a complete catalog workflow is absent.

Keep security workflow and Dependabot configuration initially; inspect and fix concrete problems without replacing them with broad skips. The old dependency-review capability bypass is not proof of dependency security. Determine actual support and run an appropriate dependency audit while correcting the probe; do not repeatedly certify an unavailable check.

## Bibliophilarr inspection

Inspected `Swartdraak/Bibliophilarr` at `b1f36b0ee417cda0b0840b60ac505ae7ad5b7aca`, including README, `IMetadataProvider`, `IMetadataProviderOrchestrator`, provider registry and Open Library provider. It exposes a provider-based path for search/lookup/refresh. The README describes Hardcover primary/Open Library secondary. This is reference context, not an instruction to import its external-source policies or branch model into BookDB.

The provider integration design is in [the integration contract](18-bibliophilarr-integration.md). A Bookshelf reference was selected as `pennydreadful/bookshelf`, a clearly identified Readarr revival; the owner did not supply a particular fork URL. Its README and Readarr's retirement notice establish metadata continuity and compatibility concerns [R13/R22](17-references.md). BookDB must not claim all forks share one compatible metadata protocol.

## Boundaries of this audit

No local user checkout, uncommitted files, database contents, actual Copilot capabilities, vLLM load, private network deployment or current account billing was accessed. No live GitHub objects were created/updated. No application tests were executed in preparing the package. Installation and S0 explicitly revalidate current state and produce actual runtime evidence.
