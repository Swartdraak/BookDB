# Autonomous agent operating model

## Authority and execution

The primary agent coordinates one ready issue and can implement it directly. Delegation is optional. An executor is a temporary bounded assignment, not a separate repository, branch or management hierarchy. A reviewer evaluates the final diff and behavior. Primary/executor/reviewer profiles are supplied under `.github/agents/`; specialist knowledge resides in the relevant documents rather than dozens of competing agents.

| Action | Standing authority |
| --- | --- |
| Inspect code/history, plan a bounded issue, edit/test application/docs | Autonomous within the active stage |
| Create/manage BookDB issues, labels, milestones, Project items and Wiki projection | Autonomous; keep canonical issue/doc identity |
| Create short-lived branch, scoped commits, push, open/update PR | Autonomous |
| Fix CI, review code, merge eligible ordinary PR, delete its safely merged branch | Autonomous after required checks/review; do not bypass platform rules |
| Deploy/restart disposable development/test services and execute stage tests | Autonomous within the named test environment |
| Advance a stage without a human gate | Autonomous when acceptance evidence passes |
| Advance S5/S7/S8 human gate | Requires real human acceptance of the candidate |
| Advance S2 (human review deferred to S5) | Autonomous after automated acceptance; S2 human procedure runs during S5 review |
| Publish GA | Allowed only after S8 human acceptance of the exact RC; no untested rebuild |
| Production data destruction, force-push shared history, disable security controls, unrequested production deployment | Not implied by routine development authority |

Security or platform permissions can still block an action. Record the concrete blocker once and continue independent ready work. Do not repeatedly ask the owner to approve actions already authorized. Do not manufacture approvals, reviewer identities or successful tests.

## Shared inference and workspace budget

- One primary agent session across GoLand, WebStorm and DataGrip. The other IDEs may expose MCP tools, but their Copilot chats must not simultaneously run separate orchestrators.
- Default: zero or one child at a time. Maximum: **two active subagents total across this backend**, not two per IDE or per repository. Other backend workloads consume capacity too.
- One writable task owner at a time. During an executor's writes, the primary does not edit/commit/switch branches and a reviewer waits for a stable diff. Read-only parallel exploration may be used only when it does not race with changing files.
- Parent yields while children use inference. Children cannot delegate, start other agents, launch background Copilot/LLM processes or change model/runtime configuration.
- No worktrees, clones/copies of BookDB for tasks, nested checkouts, or branch-per-agent schemes. References are read via GitHub or specific files. The separate GitHub Wiki is metadata documentation, not a BookDB application checkout; its bounded temporary sync is the documented exception.
- An ordinary IDE MCP call is not a subagent. It does not require another LLM session. Do not assume every exposed IDE tool must be enabled in the client at once.

The supplied Markdown policies are not a backend scheduler. JetBrains hook support and concurrency controls vary; no `max_subagents` field is invented. If the installed client cannot prevent extra automatic delegation or a shared global budget cannot be established, use the sequential primary profile with no agent-invocation tool. Enable optional delegation only after inspecting the tool list and proving it respects this budget. A file instruction cannot guarantee enforcement against an unrestricted tool runner.

On timeout/OOM/backend instability: stop spawning, let outstanding calls finish or cancel them through supported controls, reduce to sequential work, shorten the context and resume the same issue from recorded evidence. Do not reboot or reconfigure 1cat-vllm, raise parallelism, copy the repository or switch models as an automatic repair. Maximum two retries for the same failing operation without new evidence; then diagnose one concrete cause or record a blocker.

## Bounded task loop

1. Read `AGENTS.md`, the current issue, the active stage and only the relevant domain document/code.
2. Confirm canonical root, current branch and working-tree status. Preserve user changes. `git diff` is evidence; never use a destructive reset to obtain a clean workspace.
3. Pick an issue with a user-visible or API/DB-observable outcome. State acceptance and a short implementation plan. Keep one active implementation issue.
4. Implement directly or pass a child the issue, relevant paths, expected behavior, tests and no-delegation/no-worktree limits. The handoff can be in the current conversation; no lease artifact is required.
5. Test, inspect the diff and review against requirements. Review by a separate child is useful when available; otherwise perform a distinct self-review pass and label it honestly. Never represent same-account AI review as independent human approval.
6. Commit selected paths, push the ordinary branch and open/update its PR using the template. Link `Closes #N` where appropriate. Recheck head SHA and required checks before merge.
7. Merge via GitHub's normal protected-branch mechanism, update the issue/Project, clean only the merged branch, and select the next ready issue. Do not end merely because one file or subtask is finished.
8. At a real blocker or human gate, provide a concrete handoff. Do not generate a new governance task to explain why implementation has stopped.

## Context budget for the 27B backend

Initial operating targets (not claims about GPU capacity): task packet in conversation ≤1,500 words; child handoff ≤1,000 words; result summary ≤700 words; tool output usually ≤200 lines. Load narrow file ranges and search results before whole files. Split a large diff review by cohesive component and then inspect cross-component behavior. Keep the active context well below the backend's measured configured limit; no inference-capacity assumption is made from model name or parameter count.

These limits are defaults to avoid context inflation, not product test gates. If a complex change requires more context, load it deliberately and remain sequential. The 64 GB aggregate VRAM figure does not itself establish a safe number of simultaneous long-context requests.

## Preventing governance drift

No tasks for recertifying completed M0/M1, retroactively rebuilding leases, changing all agent names, creating new orchestration frameworks or polishing every document. A governance modification must name a concrete current failure and the smallest repair. After adoption, application acceptance is the work queue.

New requirements discovered while coding become issues, not opportunistic scope expansion. Changes necessary to finish the selected behavior are allowed and documented. Architectural decisions should be resolved at the smallest useful level; do not route every cross-domain feature through several ceremonial reviewers.

## Session handoff

Update one comment on the active issue/PR: current branch/head; changed files; passed/failed commands; uncommitted work; next specific action; blocker if any; active children (normally none before ending); human gate status. Do not copy a transcript or full repository contents. On resume, verify GitHub and Git state instead of trusting stale context.
