# Installation

Copy these items into the root of the BookDB repository:

```text
AGENTS.md
CLAUDE.md
.codex/
.github/
agent-kit/
prompts/
tools/
```

Do not copy `examples/` into production agent state unless desired; it exists as reference material.

Create a runtime coordination directory:

```text
.agent-state/
  tasks/
  leases/
  handoffs/
  reviews/
  ledger/
  integration/
```

Recommended `.gitignore` policy:

```gitignore
.agent-state/runtime/
.agent-state/worktrees/
```

Whether task packets and review artifacts are committed is a project decision. For high-value architecture and release work, retaining them is recommended.

## GitHub Copilot

Use `.github/copilot-instructions.md` and `.github/agents/*.agent.md`.

The wrappers intentionally defer to the canonical files under `agent-kit/`. Do not maintain divergent copies of role rules.

## Claude Code

`CLAUDE.md` tells Claude Code to load the canonical agent rules. Run the Orchestrator only when the harness actually supports delegated agents. If it does not, the Orchestrator creates task packets for separate Claude sessions and stops.

## Codex

`.codex/AGENTS.md` points Codex to the same canonical rules.

## Generic agent harness

Use `agent-kit/adapters/generic/CONTROL_PLANE_SYSTEM_PROMPT.md` for the Orchestrator and the role file corresponding to the selected specialist.

## Critical migration note

Delete or archive the previous loose agent definitions. Do not leave both v2 and v3 role files active, because contradictory instructions will weaken enforcement.

See `MIGRATION_FROM_V2.md`.
