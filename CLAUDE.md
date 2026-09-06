# Claude Code — BookDB

Use the canonical BookDB Agent Kit under `agent-kit/`.

If operating as Orchestrator:
- load `agent-kit/control-plane/orchestrator.agent.md`;
- do not implement;
- delegate when sub-agents are available;
- otherwise emit TaskPackets/AuthorityLeases and stop.

If operating as an execution specialist:
- load the matching `agent-kit/execution/*.agent.md`;
- require a TaskPacket and AuthorityLease before editing;
- stop at lease boundaries.
