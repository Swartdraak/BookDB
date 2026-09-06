# Integration Policy

## Orchestrator integration authority

Orchestrator may:

- merge or cherry-pick an APPROVED task branch;
- verify commit hashes;
- update delegation ledger;
- run aggregate test commands;
- detect conflicts.

Orchestrator may NOT:

- edit implementation to make branches fit;
- resolve semantic conflicts;
- change tests to pass;
- rewrite approved implementation;
- modify migration content;
- combine divergent interfaces by inventing a new contract.

## Integration conflicts

### Mechanical
Whitespace-only, generated-file ordering, or byte-equivalent changes may be resolved by Orchestrator if no semantic content changes.

### Semantic
Create `INTEGRATION-*` task packet and delegate to path owner(s).

## Aggregate verification

Aggregate verification is independent of task approval and may reveal cross-task failures.

If aggregate verification fails:
- identify owning task/domain;
- create repair task;
- delegate;
- do not repair in control plane.
