# Delegation Rules

## Rule D1 — implementation is always delegated

The Orchestrator MUST delegate all implementation work to an EXECUTION agent.

There is no “small enough for the Orchestrator to do directly” exception.

## Rule D2 — planning precedes implementation when required

The Orchestrator MUST route to a PLANNING_AUTHORITY agent before execution if a change involves:

- canonical domain semantics;
- new public API semantics;
- new durable event contract;
- new source persistence policy;
- new infrastructure dependency;
- new security trust boundary;
- destructive/irreversible schema behavior;
- licensing change.

## Rule D3 — one primary executor

Each task packet has exactly one primary execution agent.

Reviewers are separate.

## Rule D4 — writable path lease

Every executor receives exact path globs.

“Repository-wide as needed” is invalid.

## Rule D5 — failed delegation returns

If an executor returns incomplete/failed work:

- Orchestrator marks task `CHANGES_REQUESTED` or `BLOCKED`;
- returns it to the same executor or assigns a new executor with a new lease;
- Orchestrator MUST NOT repair it.

## Rule D6 — sub-agent capability check

At startup the Orchestrator determines:

```text
CAN_INVOKE_SUBAGENTS = true | false
```

If false:
- create task packets and leases;
- output the ordered delegation queue;
- STOP.

Do not implement as fallback.

## Rule D7 — dependency ordering

A dependent task cannot enter `IN_PROGRESS` until all hard dependencies are `APPROVED` or `INTEGRATED`, as declared by workflow.

## Rule D8 — parallel work

Parallel tasks require:
- non-overlapping write leases;
- common base commit or declared dependency;
- integration order.

Separate Git worktrees are NOT required for every task. Writable-path leases provide
logical ownership boundaries. Tasks whose paths conflict execute sequentially.
Independent tasks may execute sequentially in the canonical checkout without worktree
creation.

ISOLATED workspace mode (an external Git worktree) is used only when actual concurrent
writers, risky experiments, or explicit isolation requirements justify it. All ISOLATED
worktrees must be outside the canonical repository and removed immediately following
successful integration. See `WORKSPACE_POLICY.md`.

## Rule D9 — shared files

Shared high-contention files such as:
- `go.mod`
- `go.sum`
- `package.json`
- `pnpm-lock.yaml`
- root Compose files
- root config schema

must be owned by a designated integration task or platform executor. Multiple agents do not casually edit them in parallel.

## Rule D10 — semantic conflict

If integration causes a semantic conflict:
- identify owning domains;
- create a conflict-resolution task;
- delegate to the owning execution agent with required planning reviewers.

Orchestrator may resolve purely mechanical conflicts only when resulting content is byte-for-byte equivalent to already approved changes.

## Rule D11 — workspace mode

Every AuthorityLease (schema 1.1) declares exactly one `workspace_mode`:

- `CANONICAL` (default): operate in the canonical checkout; no new worktree.
- `ISOLATED`: external temporary worktree outside the canonical repository, only when
  genuine concurrency or risky experimentation requires isolation.
- `READ_ONLY`: planning/review agents that only inspect state; no branch or worktree.

Review and planning agents normally use `READ_ONLY` and do not receive worktrees.
No Git worktree may be created underneath the canonical repository. See
`WORKSPACE_POLICY.md`.
