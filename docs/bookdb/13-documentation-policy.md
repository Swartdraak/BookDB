# Documentation ownership and change policy

The repository should be deeply documented without making agents read or regenerate everything. Each fact has one authoritative home; other surfaces link to it.

| Information | Authority | Derived surfaces |
| --- | --- | --- |
| Product scope and requirement IDs | `docs/bookdb/01-product-requirements.md` | README, Wiki introduction |
| Architecture/data/security/source/UX contracts | Respective numbered specification | Issue excerpts and ADR rationale |
| Planned stages and acceptance | `docs/bookdb/stages.json` plus generated delivery-plan rendering | Seeded milestone/issue descriptions |
| Current work status, decisions about a task and evidence | GitHub Issue/PR | Project board, milestone progress |
| Actual supported dependencies | `go.mod`, `go.sum`, `web/package*.json`, tested image pins | Release dependency table |
| Implemented HTTP contract | Canonical OpenAPI file | API documentation and generated clients |
| Actual migration state | Versioned migrations + DB migration history | Upgrade/runbook notes |
| Durable architecture changes | Numbered ADR | Relevant specification amendment |
| Operating commands | Tested scripts/config and associated runbook | Wiki links and release instructions |
| Historical policy/evidence | Git history at pre-adoption commit/tag | Migration audit links only |

Package version 3.0.0 is the governance/documentation revision, not application version 3.0.0. Do not overwrite the application's own version to match it.

## Same-change documentation rules

- API behavior: update OpenAPI, examples/client generation and contract tests in the same PR.
- Schema/identity: forward migration, compatibility/backfill/rollback notes, data-model change and regression evidence.
- Connector: field mapping, policy terms reference, schedule/checkpoint semantics and eligible fixture.
- UI: journey/error/permission behavior and relevant test steps/screenshots; no need for a new UX essay per button.
- Config/deployment: update sample/runbook and test the documented command.
- Architecture change: one short ADR with context, decision, alternatives, consequences and superseded decision links when needed.

Documentation-only corrections are allowed. They do not require a new agent class, task lease, ceremonial certification or repo-wide format sweep. Limit review to changed active documents and the links they affect.

## Planned versus implemented

Specs use “must/required/target” for future behavior. README/release docs say what the verified version actually does. Do not call an endpoint implemented because it exists in the OpenAPI outline, or authentication complete because configuration validation exists. If a required future stage runner is missing, report NOT IMPLEMENTED.

Maintain a concise tested-commands section with actual version/environment and outputs linked to CI. Long logs live in bounded CI artifacts or operator storage, not Git. A handoff is one issue/PR comment, not a new permanent ledger.

## Reading paths and context

Default agent context: `AGENTS.md`, active issue, relevant stage and affected source files. Load a domain spec only when needed. `BOOKDB_PROJECT.md` is a navigation/decision index. Do not bulk-attach every document, old transcript or previous governance archive to a 27B model context.

## Wiki, release docs and archive

The Wiki is generated navigation to committed repository documents. Do not edit the Wiki to change requirements. Changes arrive through normal repo PRs and are then synchronized. Version-specific release instructions link to the release tag/commit; main documentation may describe future features and must not be mistaken for stable-release instructions.

The adoption manifest removes retired active instructions and redundant historical copies from the working tree, while Git preserves them at the baseline commit. Do not recreate `.agent-state` or copy old archives back for model context. Useful historical ADRs remain, with explicit amendments in ADR 0011. Do not erase valid license notices or source-policy history needed for application data.

## Document validation

Run `check_docs.py` for local file-link integrity and basic package invariants. It is intentionally small and does not judge prose length, role counts, task state or approval paperwork. External links are rechecked when their factual claims are used for a decision or when source/tool policy changes; do not make every PR depend on live access to every referenced website.

`stages.json` and the readable stage plan must be amended together if acceptance changes. GitHub is not continuously overwritten from the seed: refine existing issue bodies deliberately, preserve discussion and current status, and never reopen accepted stages simply to regenerate text.
