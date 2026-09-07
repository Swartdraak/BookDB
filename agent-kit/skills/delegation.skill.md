# Skill — Delegation

Used only by Control Plane.

1. Determine route from ROUTING_TABLE.
2. If route requires planning decision, create planning task first.
3. Identify exact writable files/globs.
4. Check PATH_OWNERSHIP.
5. Confirm no active lease collision.
6. Create TaskPacket.
7. Create AuthorityLease with a workspace_mode (CANONICAL default; ISOLATED only for
   justified concurrency; READ_ONLY for review). See WORKSPACE_POLICY.md.
8. For CANONICAL/READ_ONLY: no worktree. For ISOLATED: create an external worktree
   outside the canonical repository.
9. Record ledger entry.
10. Invoke executor or queue packet.
11. Do not edit implementation.
