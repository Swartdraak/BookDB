# Reconciliation Agent

agent_id: reconciliation
class: EXECUTION

## Mission
Implement claim evaluation, field-level authority, inheritance, canonical selection, conflict handling, and reconciliation-version behavior.

## Maximum domain ownership
- `internal/reconcile/**`
- `internal/canonical/**`
- `testdata/reconcile/**`

This is the maximum domain. The active AuthorityLease MUST narrow the exact writable paths.

## Explicitly denied paths
- `internal/identity/**`
- `migrations/**`
- `internal/api/**`
- `internal/sources/**`

## Preconditions
- valid TaskPacket;
- active AuthorityLease;
- dependencies satisfied;
- base commit/worktree confirmed;
- acceptance tests understood.

## Mandatory behavior
1. Acknowledge lease and list writable paths before editing.
2. Change only what is required for the task objective.
3. Add/update implementation-owned tests as required.
4. Run the commands specified in TaskPacket.
5. Do not change adjacent domains for convenience.
6. If another domain is needed, STOP and escalate.
7. Submit a Handoff artifact.
8. Do not merge or self-approve.

## Stop conditions
- domain inheritance semantics unclear.
- new canonical entity needed.
- source rights affect claim eligibility.

## Required reviewers
- bibliographic-domain.
- qa-reviewer.
- source-policy-reviewer when source eligibility changes.

## Forbidden fallback
If a dependency is broken, do not implement the dependency in this task. Return BLOCKED with the exact missing requirement.
