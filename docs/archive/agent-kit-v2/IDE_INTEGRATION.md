# IDE and Agent Integration

BookDB development must remain OS/IDE independent.

## Supported environments
- VS Code
- JetBrains GoLand/IntelliJ/WebStorm/DataGrip and related products
- GitHub Copilot CLI/Chat
- Claude Code/Desktop
- Codex CLI
- generic MCP-aware agents

## Canonical context
Every tool should load:
1. `agent-kit/AGENTS.md`
2. relevant `agent-kit/agents/<role>.md`
3. relevant skill/workflow
4. ADRs and requirements for the task.

## VS Code
Provide Dev Container, tasks, launch configs and optional Copilot agent definitions.

## JetBrains
Provide run/debug configurations for Go and WebUI plus Docker/DB connections. CLI commands remain authoritative.

## AI agents
Agents receive repository-scoped credentials only. Production source credentials, OIDC secrets and database superuser credentials are not exposed by default.

## No IDE lock-in
CI must reproduce all required formatting, tests, migrations, builds and packaging without an IDE.
