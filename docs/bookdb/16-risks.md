# Risk register and decision boundaries

This is a focused engineering register. Create a GitHub issue when a risk becomes actionable; do not mirror issue status here.

| Risk | Consequence | Mitigation and trigger | Owner role |
| --- | --- | --- | --- |
| Global completeness interpreted as a finite guarantee | Misleading release claims or endless scope | Snapshot accounting and format/language coverage; release capability separately from corpus coverage | Product/maintainer |
| Open-source software confused with open metadata rights | Ineligible copied fields/assets | Source-specific field/asset policy; actual terms references; isolate affected source rather than stall all delivery | Source implementer |
| Sparse contemporary audiobook coverage | Missing narrators/performances despite rich model | Measure gaps, validate an eligible real audio source, keep unknown values and quality gate honest | Data/domain |
| False merges | Loss of distinct editions/people and consumer corruption | Hard negatives, precision targets, conservative matching, revisioned reversible identity operations | Data/domain |
| Human moderation backlog | Contributions never reach public catalog | Good diff/evidence UI, source auto-publication under policy, sensible triage; never bypass administrator approval | Administrator |
| Incorrect source attribution/license on existing registry | Bad redistribution policy | Fix precise record in S2; retain source facts and rights distinctions | Source implementer |
| Legacy governance survives adoption | Agents resume leases/worktrees and drift | Exact manifest, small instruction bridge, remove old profiles/validators and reload IDE | Primary agent |
| More than two children or long concurrent contexts | Inference instability/OOM | Sequential profile default; one active IDE chat; optional bounded delegation only after verification | Developer |
| Agent instruction mistaken for hard enforcement | Hidden recursive calls/worktrees | Inspect real tool capability; disable delegation when controls unverified; no unsupported hook promises | Developer |
| Skeleton checks pass but runtime absent | “Completed” stages unusable | Explicit app binary/image build, real DB/services, stage runner failure on missing implementation | QA/implementer |
| API quota bypass across replicas/keys | Resource exhaustion | Shared atomic quotas, account aggregate caps, failure policy, bounded expensive operations | API/security |
| Unapproved content leaks via search/export/assets | Moderation/privacy failure | Shared publication eligibility and negative tests on all read paths | Security/QA |
| Event sequence skips late commit | Consumer catalog divergence | Commit-order publication/watermark tests and durable resync | Backend |
| Source snapshot huge or malformed | Memory/disk exhaustion, stuck jobs | Streaming, bounds, chunk checkpoints, backpressure, quota and accounting | Ingestion |
| Object bytes survive but filer metadata lost | Unreadable assets/raw records | Complete storage persistence/restore tests, not just Docker volume existence | Operations |
| API/client provider semantics differ | Bibliophilarr duplicates or loses tracking | Namespaced ID mappings, explicit provider, staged upgrade/dry-run and contract tests | Integration |
| GitHub permissions/feature limits differ | Automation setup stalls | Read actual scopes/settings; one capability issue; continue application work with existing tools | Maintainer |
| Cloud CI exposes homelab or consumes budget | Security/cost incident | Standard hosted PR runners; trusted isolated HA runs; artifact limits and no LLM CI fan-out | Operations |
| AI claims human approval | Unsafe stage/release advance | Human-recorded candidate signoff; gate cannot be closed by generated evidence | Maintainer |

## Assumptions made explicit

BookDB remains the project/repository name. Existing AGPL server/Apache client licensing remains accepted. Three IDEs can inspect one canonical repository; actual MCP permissions are verified once. The owner can allocate resources, so benchmark results determine capacity rather than an invented hard hardware limit. No public central catalog service or cross-instance federation is required for v1. Bibliophilarr modifications happen in its own authorized change process, not silently inside the BookDB package.

The package favors preserving current dependencies during adoption. A discovered vulnerability or unsupported release may require a focused upgrade; the manifest is not a policy to freeze insecure versions forever. Public reference sources are checked as of the research date; recheck relevant terms/tool capabilities when implementing them.
