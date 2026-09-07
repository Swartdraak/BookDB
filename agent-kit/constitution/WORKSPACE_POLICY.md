# Workspace Policy

This policy defines how agents obtain and use a working repository. It corrects the
legacy M0 model in which nearly every delegated task received a complete Git worktree,
frequently nested inside the canonical repository under `.agent-state/worktrees/**`.

That model is retired. It is preserved only as historical M0 audit evidence.

## Core principle

> **ONE canonical BookDB working repository by default, with optional temporary
> external Git worktrees only when genuine concurrent execution requires isolation.**

The canonical repository is the single working checkout:

```text
<canonical-parent>/BookDB
```

A Git worktree is an entire working directory. Creating one inside the canonical
repository nests a complete BookDB checkout under itself, causing IDE indexing
pollution, recursive file searches, duplicated package discovery, agent context
confusion, tooling ambiguity, unnecessary disk consumption, and stale worktrees
accumulating invisibly.

**No future Git worktree may be created underneath the canonical repository.**

## Workspace modes

Every AuthorityLease (schema 1.1) declares exactly one `workspace_mode`.

### CANONICAL (default)

```yaml
workspace_mode: CANONICAL
workspace: <canonical-parent>/BookDB
branch: feat/m1-<milestone-or-coherent-feature>
```

- Operates in the canonical BookDB checkout.
- No new Git worktree.
- The normal default for execution.
- Tasks remain isolated by writable-path lease, not by filesystem checkout.
- Conflicting tasks execute sequentially.
- Independent agents may operate sequentially against the active milestone branch.
- Normal commits preserve provenance.

### ISOLATED (optional)

```yaml
workspace_mode: ISOLATED
workspace: <canonical-parent>/BookDB-worktrees/<TASK-ID>
branch: agent/<TASK-ID>
```

Use only when actual concurrency or risky experimentation materially requires
isolation. Requirements:

- The worktree MUST exist OUTSIDE the canonical BookDB repository.
- The worktree MUST be temporary.
- The branch MUST be task-specific.
- The worktree MUST be removed immediately after approved integration.
- The branch MUST be deleted after merge when safe.
- `git worktree prune` MUST follow cleanup.

A suitable default external root is `<canonical-parent>/BookDB-worktrees/`. Derive it
from the canonical parent path or configuration/environment rather than hard-coding a
user home path.

### READ_ONLY

```yaml
workspace_mode: READ_ONLY
workspace: <canonical-parent>/BookDB
```

For planning agents, assurance reviewers, architecture reviewers, security reviewers,
documentation reviewers, and any agent that only inspects repository state.

- No branch or worktree creation is required merely to review existing content.
- No write operations are permitted.

## Pre-edit authority assertion

Executors MUST validate their workspace against the lease before editing. The legacy
assertion `CURRENT_WORKTREE == AuthorityLease.worktree` is retired.

### CANONICAL

```text
CURRENT_WORKSPACE == AuthorityLease.workspace
CURRENT_BRANCH    == AuthorityLease.branch
```

### ISOLATED

```text
CURRENT_WORKSPACE == AuthorityLease.workspace
CURRENT_BRANCH    == AuthorityLease.branch
workspace is outside the canonical repository
workspace is registered in `git worktree list`
```

### READ_ONLY

```text
CURRENT_WORKSPACE == canonical repository
no write operations permitted
```

A Git worktree is never required merely to satisfy the assertion.

## Parallel execution policy

### Sequential execution (default)

When agents share the canonical workspace, they execute sequentially. This is the
normal and acceptable default. Example:

```text
backend-api  -> commit
database     -> commit
frontend     -> commit
```

Each agent retains its TaskPacket, AuthorityLease, path ownership, agent attribution,
Handoff, and ReviewReport. A separate filesystem checkout is not necessary for
provenance.

### True parallel execution

Only when two or more agents actually need to write simultaneously:

```text
ISOLATED workspace + external worktree + task branch
```

Do not parallelize merely because the system can. Prefer simpler canonical execution
unless concurrency materially improves throughput.

## Branch model

Default:

```text
main
  └── feat/m1-<milestone-or-coherent-feature>
          ├── delegated executor commits
          ├── reviews
          ├── fixes
          └── PR -> main
```

Task-specific branches exist only when justified by:

- concurrent isolated execution;
- risky work;
- an independent PR need;
- a specific integration concern.

Do not create a new branch solely because a TaskPacket exists.

## Cleanup requirements

- Remove an ISOLATED worktree immediately after successful integration.
- Delete the merged task branch when safe.
- Run `git worktree prune` after cleanup.
- The steady-state `git worktree list` should show only the canonical repository
  unless an explicitly justified active isolated workspace exists.

## Historical M0 legacy treatment

M0 used the legacy workspace model: a Git worktree per task, nested under
`.agent-state/worktrees/**`. M0 was a recovery exercise and its artifacts are
retrospective audit evidence.

- M0 leases (schema 1.0, `worktree:` field) remain valid and auditable.
- M0 worktree paths in historical records are preserved as-is.
- M0 leases do NOT represent current best practice and MUST NOT be used as a template
  for M1+ work.
- M1+ uses the corrected workspace-mode model (schema 1.1).

## Retrospective provenance policy

Retrospective certification (e.g. `M0-RETRO-*-PROVENANCE-*`) is an exceptional recovery
mechanism for inherited or unmanaged historical work. It MUST NOT substitute for ordinary
prospective `TaskPacket -> Lease -> Execution -> Handoff -> Review` provenance. M1 and
future milestones use prospective provenance.

