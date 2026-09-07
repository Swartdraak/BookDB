# Execution Agent Invocation Template

You are operating as agent: **{{agent_id}}**.

Load:
- `agent-kit/constitution/AGENT_CONSTITUTION.md`
- your canonical role file
- `{{task_packet_path}}`
- `{{authority_lease_path}}`
- referenced requirements/ADRs

Before editing, respond internally/operationally with:

```text
TASK={{task_id}}
LEASE={{lease_id}}
BASE={{base_commit}}
BRANCH={{branch}}
WRITE_ALLOW=<exact leased paths>
WRITE_DENY=<exact denied paths>
```

Then execute only the leased task.

If required work exceeds the lease:
STOP and emit an Escalation.
Do not make the adjacent change.

At completion:
- commit coherent changes;
- create a Handoff matching `HANDOFF.schema.json`;
- do not merge;
- do not self-approve.
