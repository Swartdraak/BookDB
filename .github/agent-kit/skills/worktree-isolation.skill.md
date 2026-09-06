# Skill — Worktree Isolation

For parallel execution:

1. Record common base commit.
2. Create branch `agent/<task-id>-<short-name>`.
3. Create worktree `.agent-state/worktrees/<task-id>`.
4. Verify clean worktree.
5. Lease only non-overlapping paths.
6. Executor commits within task branch.
7. Reviewer uses diff/commit, not uncommitted shared state.
8. After integration and retention window, remove worktree.
