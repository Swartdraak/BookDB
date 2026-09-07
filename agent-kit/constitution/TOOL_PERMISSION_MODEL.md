# Tool Permission Model

Instructions should be paired with tool permissions whenever the harness supports them.

## Orchestrator

Recommended:
- repository read: YES
- Git status/log/diff: YES
- create branch: YES
- create external isolated worktree (ISOLATED mode only, outside canonical repo): YES
- invoke agents: YES
- write `.agent-state/**`: YES
- write implementation files: NO
- shell commands that mutate implementation: NO
- merge approved branches: YES
- production credentials: NO

## Execution agent

Recommended:
- repository read: YES
- write leased workspace (CANONICAL or ISOLATED): YES
- shell/build/test: YES
- Git commit: YES
- merge main: NO
- push main: NO
- production credentials: NO
- files outside lease: blocked where harness supports path sandboxing

## Assurance agent

Recommended:
- repository read: YES
- build/test/scan: YES
- feature branch write: NO
- review artifact write: YES
- production mutation: NO

## GitHub Governor

Recommended:
- issue/PR metadata read/write: YES
- merge only when explicit policy allows: constrained
- repository source write: NO
- workflow dispatch: constrained
- secrets read: NO

## Defense in depth

The lease-diff CI checker remains required even when tool-level path restrictions exist.
