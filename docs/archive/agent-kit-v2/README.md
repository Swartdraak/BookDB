# BookDB Agent Kit v3

**Purpose:** provide an operational, tightly constrained multi-agent development system for BookDB.

This package replaces the loosely scoped v2 role catalog. v3 treats agent coordination as an explicit control system with:

- four agent classes;
- hard domain and path ownership;
- a control-plane Orchestrator that is prohibited from implementing application code;
- machine-readable task authority leases;
- deterministic task routing;
- worktree/branch isolation;
- dependency-aware delegation;
- mandatory independent review;
- task and review state machines;
- escalation and stop conditions;
- structured handoff contracts;
- GitHub governance rules;
- harness adapters for GitHub Copilot, Claude Code, Codex, and generic agent systems;
- validation tooling for task packets, authority leases, handoffs, reviews, routing tables, and ownership maps;
- Git-diff authority-lease enforcement so CI can reject out-of-scope agent edits.

## Fundamental rule

> **The Orchestrator coordinates work. It does not perform implementation work.**

If the current agent harness cannot invoke sub-agents, the Orchestrator MUST create valid task packets for the required execution agents, place them in the delegation queue, and stop. It MUST NOT implement the work itself as a fallback.

## Agent classes

| Class | Purpose | May implement application code? |
|---|---|---:|
| Control Plane | orchestration, routing, repository governance | No |
| Planning / Authority | requirements, architecture, bibliographic decisions, source policy | No, except their own decision artifacts |
| Execution | bounded implementation within leased paths | Yes |
| Independent Assurance | independently verify implementation and gates | No feature repair |

## Install

Read `INSTALL.md`.

## Start development

Read:

1. `agent-kit/constitution/AGENT_CONSTITUTION.md`
2. `agent-kit/constitution/AUTHORITY_MODEL.md`
3. `agent-kit/constitution/DELEGATION_RULES.md`
4. `agent-kit/routing/ROUTING_TABLE.yaml`
5. `agent-kit/routing/PATH_OWNERSHIP.yaml`
6. `agent-kit/control-plane/orchestrator.agent.md`
7. `agent-kit/workflows/repository-bootstrap.workflow.yaml`

Then invoke the **Orchestrator** with `prompts/M0_ORCHESTRATOR_KICKOFF.md`.

## Validation

```bash
python tools/validate_agent_kit.py
```

The validator checks JSON/YAML syntax, example contracts, ownership overlap, route references, and required agent metadata.
