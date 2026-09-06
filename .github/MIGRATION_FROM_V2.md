# Migration from Agent Kit v2

## Why replacement is required

v2 described responsibilities but did not adequately constrain authority. In particular:

- Orchestrator was allowed to behave as an implementation lead.
- Filesystem ownership was not explicit.
- Delegation was guidance rather than a state transition.
- Specialists had no formal task leases.
- Reviewers could drift into implementation repair.
- Failure of a delegate had no mandatory return/re-delegate behavior.
- Parallel agents had no worktree isolation contract.
- Cross-domain changes were not forced into separate work packages.
- Handoffs were unstructured.
- No machine validation existed.

## Migration procedure

1. Archive old `agent-kit/`.
2. Install v3.
3. Replace repository-level Copilot/Claude/Codex instructions with v3 wrappers.
4. Run `python tools/validate_agent_kit.py`.
5. Create the initial `.agent-state/` directories.
6. Start a fresh Orchestrator session.
7. Do not reuse a v2 Orchestrator conversation as the v3 control-plane process.

## Behavioral difference

### v2
“Orchestrator integrates and verifies final changes.”

### v3
“Orchestrator may merge/cherry-pick already approved delegated branches. It cannot alter implementation semantics, repair code, create tests for failed tasks, or resolve semantic merge conflicts. Those actions are re-delegated to the path owner.”

This restriction is deliberate.
