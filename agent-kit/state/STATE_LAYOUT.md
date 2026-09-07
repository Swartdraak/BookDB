# Runtime State Layout

Recommended:

```text
.agent-state/
  objectives/
  tasks/
    <task-id>.yaml
  leases/
    <lease-id>.yaml
  handoffs/
    <task-id>/<attempt>.yaml
  reviews/
    <task-id>/<review-id>.yaml
  ledger/
    <objective-id>.yaml
  escalations/
  integration/
  runtime/
```

Durable governance state (`objectives/`, `tasks/`, `leases/`, `handoffs/`, `reviews/`,
`ledger/`, `escalations/`, `integration/`) is tracked normally in Git as audit evidence.

Only ephemeral/runtime state (`runtime/`, `tmp/`, `cache/`, `logs/`) is ignored by Git.

Git worktrees are NOT stored under `.agent-state/`. ISOLATED worktrees live outside the
canonical repository (see `agent-kit/constitution/WORKSPACE_POLICY.md`). The historical
`.agent-state/worktrees/` path is retired and must not be reused.

Task/review artifacts can be retained in Git for important work or archived in CI artifacts.
