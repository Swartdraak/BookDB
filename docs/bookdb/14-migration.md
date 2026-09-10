# Exact adoption and replacement instructions

This package is a reviewed overlay and a hash-checked migration plan for the existing BookDB repository. It preserves application code and Git history. Do not drag/drop the overlay on top of main or delete broad directories by memory.

## What changes where

| Existing path/group | Action | Replacement authority |
| --- | --- | --- |
| `AGENTS.md`, `.github/copilot-instructions.md` | Replace | Small executable-workflow rules and optional delegation |
| `.codex/AGENTS.md`, `CLAUDE.md` | Replace with pointers | Root entry point; no competing agent constitution |
| Existing `.github/agents/*.agent.md` | Retire all snapshot-listed profiles | Four focused profiles: sequential primary, optional delegating primary, executor, reviewer |
| `agent-kit/**` | Remove snapshot-listed tracked files from active checkout | `docs/bookdb/10-agent-operation.md` and GitHub issue/PR lifecycle |
| `.agent-state/**` | Retire tracked historical ledgers/leases/packets/reviews | Git history retains evidence; GitHub becomes live task authority |
| `prompts/**`, old governance examples | Retire exact listed paths | Package kickoff prompt and stage issue acceptance |
| `docs/archive/agent-kit-v2/**` | Retire duplicate archive files from checkout | Baseline Git commit remains historical archive |
| Old numbered planning/system docs, master plan, `project-management/**`, old handoff/bootstrap/manifest files | Retire snapshot-listed superseded documents | Root `BOOKDB_PROJECT.md` + focused `docs/bookdb/**` |
| `docs/24_LICENSING_POLICY.md`, `LICENSE*`, `LICENSES/**`, `SDK_LICENSE.md`, `THIRD_PARTY_NOTICES.md`, `CODE_OF_CONDUCT.md` | Preserve | Existing licensing/contribution notices, amended only through a reviewed need |
| `adrs/0001`–`0010` | Preserve history | New ADR 0011 explicitly amends governance/sequencing/identity interpretation |
| `.github/workflows/agent-governance.yml` | Retire | Small docs check plus actual build/test gates; no leases |
| `tools/check_lease_diff.py`, `validate_agent_kit.py`, `validate_contract.py`, `test_workspace_model.py`, `requirements-agent-kit.txt` | Retire | No replacement authority/lease validator |
| `.github/workflows/ci.yml` | Replace, retaining Go/Web checks and adding runtime image/docs jobs | Tested application behavior and active docs |
| `.github/workflows/security-supply-chain.yml`, `compose-smoke.yml`, `dependabot.yml` | Preserve, inspect follow-up corrections | Do not discard security/application tests while removing governance |
| `.github/ISSUE_TEMPLATE/**`, PR template, CODEOWNERS | Replace snapshot-listed template content | Issue types, acceptance/evidence and actual maintainer ownership |
| README/CONTRIBUTING/SECURITY | Replace navigation/current policy text | New handbook, test/operation/security specifications |
| Historical deployment planning Markdown | Replace with short pointers | Current operations and architecture documents; actual Compose/runtime files preserved |
| `internal/**`, `web/**`, `api/**`, `config/**`, Go manifests, Docker/Compose/devcontainer/application scripts | **No installer edits** | Repair only in implementation PRs with tests |

`migration-manifest.json` in the outer package contains the **exact paths and original Git blob hashes**, plus every overlay destination. `MIGRATION_PATHS.md` is its human-readable listing. Directory names above summarize that list; the installer does not recursively delete arbitrary directory contents.

## Adoption sequence

1. Place the extracted package outside the repository. Close/stop other active agent sessions. Open the current canonical repository; do not clone another copy.
2. Inspect `git status --short`, current branch/remotes and `git worktree list`. Preserve local user work. Do not reset, stash automatically or remove an unknown worktree. The installer requires a clean tracked/untracked state so unrelated work cannot be overwritten.
3. Read current GitHub main/issues/PRs. Record the live main SHA. Confirm the audited baseline is in history; if new commits exist, assess affected paths. The installer compares touched paths, not every application file.
4. Fetch current origin and fast-forward a clean main using normal Git commands; create `chore/<issue>-adopt-project-package` (or `chore/adopt-project-package` before the issue exists). If already on a suitable feature branch, reuse it. Never create an extra worktree.
5. Preview the migration:

```bash
python3 /absolute/path/BookDB-Project-Package/install.py --repo /absolute/path/BookDB
```

6. Review the output and exact manifest. Apply on the feature branch:

```bash
python3 /absolute/path/BookDB-Project-Package/install.py --repo /absolute/path/BookDB --apply
```

The installer refuses main/master, a dirty checkout, a mismatched remote, changed expected files, symlink paths, untracked overwrite targets, missing historical baseline or missing/mismatched overlay content. It stages/commits nothing and performs no GitHub/API/database operation. It is idempotent for the intended before/after file contents, but the first application naturally leaves a dirty diff for review.

If a touched file changed since inspection, do not use force or edit the manifest hash merely to suppress the conflict. Inspect that file, preserve useful changes and reconcile the replacement manually under the authorized adoption task. Record what was retained. The migration manifest is a safety check, not a new governance gate for future work.

7. Run active docs validation. Inspect `git diff --stat`, `git diff --name-status`, and the changed templates/workflow/instructions. Confirm application source/config bytes are unchanged by installation. Review removed files through `git show <baseline>:<path>` only as needed; do not create a new active archive.
8. Add `/.local/` and any local acceptance output to the existing `.gitignore` without removing useful rules. Review workspace/IDE-level instructions outside the repo for obsolete lease/worktree requirements and replace those references if present. Do not rewrite unrelated personal instructions.
9. Complete the concrete S0 runtime repairs: restore `cmd/bookdb`, wire real migrations/config, replace ordinary `compose-down` volume deletion, and prove actual binary/image startup and persisted restart. Add the S0 acceptance runner. This is implementation work, not another documentation audit.
10. Run existing and new required CI locally/through the PR. Configure GitHub seed objects with the supplied script. Exchange any obsolete required governance contexts for the new checks without weakening security rules. Create one adoption/runtime PR linked to its issue; review and merge normally.
11. Reopen/reload Copilot customizations so old profiles are no longer cached. Select the sequential primary profile. Prove the capability checks, then continue S1 automatically. Do not spend another stage on agent-kit certification.

## Rollback

Before commit: inspect the migration diff and restore only the known migration paths from the recorded pre-adoption commit if rollback is needed; preserve other work. After merge: revert the adoption commit through a normal PR. Application changes need their own rollback analysis; this installer does not alter databases. If the adoption PR includes runtime repairs, evaluate them before reverting the whole PR. Never use a full repository reset as the generic rollback instruction.

## Verify obsolete rules are inactive

Search current instruction/template/workflow paths for `AuthorityLease`, `TaskPacket`, `agent-kit/`, `.agent-state/`, `validate_agent_kit`, and worktree-creation requirements. References in the migration/audit/ADR that explain retirement are expected; active requirements are not. Do not enforce a blanket ban on those words in all documentation. Verify one canonical checkout, no cached old agent selection, a normal branch/PR and actual passing runtime tests.

The package's test report covers its installer/checker behavior in disposable fixtures, not this live adoption. The user's agent must report the real adoption result honestly.
