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
  worktrees/
```

`runtime/` and `worktrees/` should normally be ignored by Git.

Task/review artifacts can be retained in Git for important work or archived in CI artifacts.
