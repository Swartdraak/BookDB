# File Ownership Policy

Canonical ownership is machine-readable in:

`agent-kit/routing/PATH_OWNERSHIP.yaml`

Rules:

1. Execution agents edit only leased paths.
2. Ownership map defines the maximum possible domain, not automatic permission.
3. A task lease narrows ownership.
4. Shared root dependency files require an explicit `platform-devex` or integration task.
5. Planning/assurance agents cannot use ownership overlap as a reason to implement production code.
6. If a path is unowned, Orchestrator routes to System Architect for ownership assignment before implementation.
