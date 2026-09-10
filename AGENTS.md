# BookDB agent entry point

Follow the owner's current request, this file, the active GitHub issue and the relevant specification linked from `BOOKDB_PROJECT.md`. This package supersedes the retired task-packet/lease system. Do not reload old agent-kit history as current policy.

- Deliver a tested application behavior in the active S0–S8 stage. GitHub Issues/PRs own status and evidence; no `.agent-state`, TaskPacket, AuthorityLease or certification paperwork.
- Use the single canonical checkout and a normal issue branch → scoped commits → PR → required CI/review → merge. Do not create worktrees, clones or full repository copies. Preserve unrelated user changes.
- One active primary across all IDEs. Default sequential execution. Maximum two active children across the shared backend, one writer, no recursive delegation. If control of this budget is uncertain, do not delegate.
- Primary may implement directly; unavailable delegation is not a reason to stop. Use the user's selected network 1cat-vllm model; no automatic cloud fallback or inference reconfiguration.
- Existing standing authority covers routine BookDB issue/Project/Wiki/branch/commit/push/PR work and eligible merges. Stop at the actual human acceptance gates S2/S5/S7/S8 with a testable candidate and consolidated instructions. Never fabricate human approval.
- PostgreSQL owns canonical identity/publication; source records are evidence. Unknown metadata stays unknown. Consumer libraries are never source inputs. User proposals stay private until Administrator approval.
- Read narrowly: active issue, relevant stage, relevant domain spec and affected code. Do not repeatedly scan or rewrite all docs.
- Run meaningful tests. A missing runner, unavailable service, skeleton or skipped required test is not success. Use the existing command table in `docs/bookdb/08-testing.md`; implement each stage runner with its feature.
- Review final diff, commit selected paths and verify current-head checks before merge. Continue the next ready issue until a real blocker or human gate. Use one concise issue/PR handoff when ending a session.

Detailed workflow: `docs/bookdb/10-agent-operation.md`. Adoption: `docs/bookdb/14-migration.md`.
