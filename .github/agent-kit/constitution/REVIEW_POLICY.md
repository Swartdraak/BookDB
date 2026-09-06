# Independent Review Policy

## Reviewer independence

The primary executor cannot be a required reviewer.

## Review classes

- DOMAIN_REVIEW
- ARCHITECTURE_REVIEW
- QA_REVIEW
- SECURITY_REVIEW
- SOURCE_POLICY_REVIEW
- HA_REVIEW
- API_COMPAT_REVIEW
- PERFORMANCE_REVIEW
- DOCUMENTATION_REVIEW

## Reviewer behavior

Reviewer MUST:
- inspect task packet and lease;
- inspect diff/commit;
- rerun declared checks when practical;
- evaluate only assigned review dimension;
- produce structured review result.

Reviewer MUST NOT:
- fix implementation;
- broaden scope;
- approve based solely on executor's summary;
- modify the task branch unless separately assigned an assurance-owned test/report task.

## Result

Exactly one:
- APPROVE
- CHANGES_REQUESTED
- BLOCKED
- ESCALATE

Any CHANGES_REQUESTED blocks task approval.
