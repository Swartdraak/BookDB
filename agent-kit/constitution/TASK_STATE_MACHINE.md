# Task State Machine

Valid states:

```text
DRAFT
TRIAGED
SCOPED
DECISION_REQUIRED
DECOMPOSED
READY_TO_LEASE
LEASED
IN_PROGRESS
BLOCKED
HANDOFF_SUBMITTED
REVIEW_PENDING
CHANGES_REQUESTED
APPROVED
INTEGRATION_PENDING
INTEGRATED
VERIFYING
DONE
CANCELLED
ESCALATED
```

## Required transitions

### DRAFT -> TRIAGED
Requires:
- task class identified;
- affected domains identified.

### TRIAGED -> SCOPED
Requires:
- objective;
- explicit in-scope/out-of-scope;
- candidate paths;
- acceptance criteria.

### SCOPED -> DECISION_REQUIRED
Use when planning/authority decision is missing.

### SCOPED -> DECOMPOSED
Requires no missing decision.

### DECOMPOSED -> READY_TO_LEASE
Requires:
- one executor per child task;
- write paths;
- dependencies;
- required reviewers;
- verification commands.

### READY_TO_LEASE -> LEASED
Requires valid authority lease.

### LEASED -> IN_PROGRESS
Executor acknowledges task and confirms scope.

### IN_PROGRESS -> BLOCKED
Executor cannot continue without external decision/dependency.

### IN_PROGRESS -> HANDOFF_SUBMITTED
Requires handoff artifact.

### HANDOFF_SUBMITTED -> REVIEW_PENDING
Orchestrator validates handoff structure, not implementation correctness.

### REVIEW_PENDING -> CHANGES_REQUESTED
Any required reviewer rejects.

### REVIEW_PENDING -> APPROVED
All required reviewers approve.

### CHANGES_REQUESTED -> LEASED
New or renewed execution lease.

### APPROVED -> INTEGRATION_PENDING
All dependency and branch requirements met.

### INTEGRATION_PENDING -> INTEGRATED
Approved change merged/cherry-picked without semantic alteration.

### INTEGRATED -> VERIFYING
Aggregate tests begin.

### VERIFYING -> DONE
Milestone/task aggregate verification passes.

### Any -> ESCALATED
Use for authority conflict, security/legal issue, irreconcilable architecture conflict.

## Forbidden transition

`BLOCKED -> IMPLEMENTED_BY_ORCHESTRATOR` does not exist.
