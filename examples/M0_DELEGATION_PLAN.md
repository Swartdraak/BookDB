# Example M0 Delegation Plan

This demonstrates how the Orchestrator should behave.

## Planning tasks

### M0-ARCH-001
Agent: system-architect  
Purpose: freeze repository/process role boundaries and bootstrap ADR decisions.  
No implementation.

## Execution tasks

### M0-PLATFORM-001
Agent: platform-devex  
Paths: Dev Container, root manifests, dev Compose, task tooling.

### M0-CI-001
Agent: devops-release  
Dependency: M0-PLATFORM-001 interfaces frozen.  
Paths: `.github/workflows/**`, container build skeleton.

### M0-API-001
Agent: backend-api  
Paths: `internal/api/**`, `api/openapi/**`.

### M0-EVENTS-001
Agent: distributed-systems  
Paths: `internal/events/**`, `schemas/events/**`.

### M0-DB-001
Agent: database  
Paths: migration framework and `internal/database/**`.

### M0-AUTH-001
Agent: auth-identity  
Dependency: DB interface contract if persistence required.  
Paths: `internal/auth/**`, `internal/rbac/**`.

### M0-WEB-001
Agent: frontend  
Dependency: API skeleton contract.  
Paths: `web/**`.

### M0-DOCS-001
Agent: documentation-writer  
Runs after implementation contracts stabilize.

## Assurance

Each implementation task receives independent reviewers according to routing table.

## What Orchestrator writes

Only:
- `.agent-state/tasks/*.yaml`
- `.agent-state/leases/*.yaml`
- `.agent-state/ledger/M0.yaml`
- integration/report artifacts

It does not create the Go module, API handler, Compose file, migration, React app, or CI workflow itself.
