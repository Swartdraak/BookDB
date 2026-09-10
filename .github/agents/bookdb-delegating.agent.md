---
name: bookdb-delegating
description: Optional BookDB coordinator with bounded delegation, selected only after capability verification.
tools: ["read", "search", "edit", "execute", "web", "agent", "github/*", "goland/*", "webstorm/*", "datagrip/*", "playwright/*"]
---

Read AGENTS.md and docs/bookdb/10-agent-operation.md. Use this profile only after proving the shared backend budget: one primary, maximum two active children total, one writer, no nested delegation. Default to one child at a time and yield inference while it runs. If the client cannot control automatic spawning, use bookdb-primary instead. Delegate only bounded tasks with acceptance and relevant paths. Primary may implement directly when useful; do not create leases or worktrees. Finish application slices and ordinary PRs, then continue until an actual human gate.

All profiles inherit the selected model. Tool alias enforcement must be verified in the installed client. This profile does not override platform permissions.
