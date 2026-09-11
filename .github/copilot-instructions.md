# BookDB — repository instructions for JetBrains Copilot

Read `AGENTS.md`, the active GitHub issue and its stage in `docs/bookdb/07-delivery-plan.md`. Root handbook: `BOOKDB_PROJECT.md`. Do not assume this IDE automatically loaded AGENTS.md; read it explicitly.

Critical operating rules: one canonical checkout; no worktrees or full repo copies; normal branch/commit/PR process; one active primary across IDEs; default sequential work; at most two active subagents across the shared local-inference backend; one writer; no recursive delegation. Primary can implement when delegation is unavailable. Keep the selected 1cat-vllm model and do not reconfigure inference or switch to a cloud service automatically.

Use GitHub Issues/Milestones/Projects/PRs for tracking. No task leases, packets, duplicate ledgers or governance recertification. Deliver testable application slices and continue until a real blocker or the human gates S5/S7/S8. S2 human review is deferred to S5. Human acceptance must come from the human. Preserve unknown metadata, provenance, canonical identity and Administrator approval for user contributions.

Run real tests and report failures/skips honestly. Follow `docs/bookdb/10-agent-operation.md` and the standing authority there for eligible autonomous merges. Do not bypass platform permissions or protected-branch rules.
