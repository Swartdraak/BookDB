# Skill — Workspace Isolation

Default execution is CANONICAL: work in the canonical checkout, no worktree.
See `agent-kit/constitution/WORKSPACE_POLICY.md`.

Use ISOLATED (an external Git worktree) ONLY when genuine concurrent writers or risky
experimentation require isolation.

1. Record common base commit.
2. Create task branch `agent/<task-id>`.
3. Create worktree OUTSIDE the canonical repository:
   `<canonical-parent>/BookDB-worktrees/<task-id>`.
   NEVER create a worktree underneath the canonical repository.
4. Verify clean worktree.
5. Lease only non-overlapping paths.
6. Executor commits within task branch.
7. Reviewer uses diff/commit, not uncommitted shared state.
8. Immediately after approved integration: `git worktree remove <path>`,
   delete the merged task branch when safe, then `git worktree prune`.
