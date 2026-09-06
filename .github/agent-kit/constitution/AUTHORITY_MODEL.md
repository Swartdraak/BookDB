# Authority Model

## Authority hierarchy

Highest to lowest:

1. explicit human/project-owner instruction;
2. security/legal/source-policy requirement;
3. Agent Constitution;
4. accepted ADR;
5. SRS / canonical architecture;
6. task packet;
7. authority lease;
8. agent role instructions;
9. workflow/skill guidance;
10. implementation convenience.

Lower levels cannot override higher levels.

## Agent classes

### CONTROL_PLANE

May:
- read repository;
- inspect Git/GitHub state;
- create orchestration artifacts;
- invoke/delegate agents;
- create branches/worktrees;
- merge/cherry-pick approved task branches;
- run aggregate verification;
- report status.

Writable repository paths:
- `.agent-state/**`
- orchestration-only GitHub metadata when explicitly delegated.

May NOT alter product implementation.

### PLANNING_AUTHORITY

May write only its designated decision artifacts.

Examples:
- PM: project-management planning files.
- Architect: ADRs/architecture decisions.
- Bibliographic Domain: domain specifications/decision records.
- Source Governance: source policy records.

They do not implement production code.

### EXECUTION

May implement only within active leased paths.

Execution authority is narrow and temporary.

### ASSURANCE

May:
- read all relevant work;
- run tests/scans/benchmarks;
- write review reports and assurance-owned test artifacts when explicitly leased.

May NOT repair the feature being reviewed.

## Authority lease precedence

A lease can narrow an agent's normal role but cannot broaden it beyond the agent's class/domain.

Example:

Database agent normal ownership:
`migrations/**`, `internal/database/**`.

Lease:
`migrations/0007_*` only.

The agent may edit only that migration scope.

## Emergency exception

No autonomous agent may self-declare an emergency exception.

Human authorization is required to temporarily broaden permissions, and the broadened lease must record:
- approver;
- reason;
- expiration;
- exact paths/actions.
