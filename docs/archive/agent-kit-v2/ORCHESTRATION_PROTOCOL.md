# Agent Orchestration Protocol

## Orchestrator lifecycle

1. Intake issue/request.
2. Load mandatory project context.
3. Classify affected domains.
4. Determine whether ADR is required.
5. Build dependency graph.
6. Assign lead agent per work package.
7. Assign required reviewers.
8. Prevent overlapping edits when agents run concurrently.
9. Integrate changes.
10. Run complete verification matrix.
11. Prepare PR/update project state.

## Task packet

Every delegated task includes:
- issue ID;
- objective;
- in-scope/out-of-scope;
- files/modules;
- acceptance criteria;
- constraints;
- dependencies;
- required tests;
- mandatory reviewers;
- expected handoff.

## Agent response contract

Each specialist returns:
- findings;
- decisions/assumptions;
- changes;
- tests/results;
- unresolved concerns;
- files;
- recommended next task.

## Conflict resolution

Priority:
1. security/legal/source policy;
2. explicit owner/product requirement;
3. accepted ADR;
4. SRS;
5. domain invariant;
6. implementation convenience.

Agents do not independently overrule higher-priority artifacts. They open an ADR/issue.

## Parallelism

Safe examples:
- API DTO + WebUI mock based on frozen OpenAPI;
- source connector + independent fixtures;
- documentation + test harness.

Unsafe examples:
- two agents editing same migration;
- two agents changing scoring thresholds;
- architecture and implementation diverging without frozen ADR.

## Stop/escalate conditions

Escalate when:
- source terms unclear;
- destructive migration appears necessary;
- user-publication gate would be weakened;
- security invariant conflicts with feature;
- reconciliation benchmark regresses materially;
- task requires production secrets;
- requested change contradicts accepted owner decision.
