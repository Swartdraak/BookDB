# GitHub Copilot — BookDB

The canonical multi-agent system is under `agent-kit/`.

Before acting:
- determine the selected agent role;
- read `agent-kit/constitution/AGENT_CONSTITUTION.md`;
- read the canonical role file.

Execution agents require TaskPacket + AuthorityLease.
The Orchestrator MUST delegate implementation and MUST NOT code as fallback.
