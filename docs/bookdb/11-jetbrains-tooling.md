# JetBrains, Copilot and MCP setup

Environment supplied by owner: Ubuntu 24.04 laptop; current GoLand, WebStorm, DataGrip and Copilot plugins; network OpenAI-compatible `/v1` endpoint served by 1cat-vllm; qwen3.8-27b-fp8 on two Tesla V100 32 GB GPUs. Preserve the working configuration. This package does not independently validate the model identifier, GPU compatibility or the private endpoint.

## Recommended minimal toolset

| Tool | Purpose | Cost/limits and fallback |
| --- | --- | --- |
| Existing JetBrains IDEs and Copilot Chat | Main authoring surface | Use current entitlements; local inference does not establish unlimited Copilot host/service entitlement |
| Built-in IDE MCP servers | Go inspections/run configs, web inspection, database schema/query tools | Use capabilities included in the installed entitlement; no separate paid AI service assumed |
| Official GitHub MCP server | Repository/issue/PR/Actions operations | Open-source local server available; remote OAuth route documented; GitHub permissions/rate limits still apply |
| GitHub CLI (`gh`) | Exact repository, Projects and Wiki management where MCP lacks an operation | Free CLI; authenticated account scopes required; no custom GitHub management MCP needed |
| Microsoft Playwright MCP | Interactive browser investigation and E2E authoring | Open-source server; run locally; committed Playwright tests remain the regression authority |
| Terminal tools | Go, npm, Git, Docker Compose, PostgreSQL client, curl, Python | Deterministic fallback without another AI service |

Do not require paid design tools, hosted memory, paid search/MCP aggregators or a premium component library. Optional tools are adopted only when they solve a measured gap and their free allowance is adequate. Standard public-repository Actions usage has a free path, but artifact/cache/package storage and other runner classes require explicit usage budgeting [R15]. Do not promise unlimited external services.

## Connect the three IDEs to one agent

1. Open the same canonical BookDB root in GoLand and WebStorm; configure DataGrip against the intended development/test databases. Do not import a second full repository copy.
2. In each IDE, open **Settings → Tools → MCP Server**, enable the server, inspect **Exposed Tools**, and copy that IDE's own supported configuration. Use unique client names `goland`, `webstorm`, `datagrip` so one configuration does not overwrite another. Endpoints, ports and project paths come from the running IDE; this package does not hardcode them.
3. In the one active Copilot Chat, use its MCP configuration UI and combine those entries with GitHub and optional Playwright. Do not substitute VS Code configuration paths or assume that a cloud-agent `mcp-servers` frontmatter applies to JetBrains.
4. Confirm each tool route reaches the expected project/database. Pass the explicit project path where supported. A successful tool listing is not proof that SQL execution, run configurations or write operations work.
5. Limit enabled tools to the current task. Prefer GoLand for Go navigation/inspection, WebStorm for frontend inspection, DataGrip for schema/SQL, and terminal/CI for authoritative builds/tests. Keep the other IDEs open if their MCP servers require a running IDE.

JetBrains documents integrated MCP setup and exposed tools, including database-specific tools with plugin/entitlement conditions [R01–R02]. Use a database account restricted to read-only access for normal DataGrip inspection; development migrations run with a separate role. Missing/free-tier-restricted database tools are not blockers: use `psql` and committed SQL scripts. Do not purchase another AI subscription to obtain a convenience tool.

## Copilot configuration example

`docs/bookdb/mcp.github.example.json` contains the documented remote GitHub OAuth entry using the JetBrains `servers` container [R04]. Copy it through the installed plugin's configuration editor, then merge the actual IDE-generated entries. Authentication is interactive once; credentials stay in the client/OS secret store. Do not commit PAT values or private model URLs with embedded credentials.

For Playwright, install a tested pinned `@playwright/mcp` version in your tooling environment and configure its actual executable through Copilot's UI. The upstream example uses npm execution; do not use a moving `latest` package in repeatable project setup. Record the selected version once. No vision-capable model is required for accessibility-tree browser operations [R05]. Screenshots remain useful for human visual review.

## Instructions and custom agents

GitHub documents `.github/copilot-instructions.md` for JetBrains repository instructions and supports additional customization surfaces with version/preview qualifications [R03]. The package keeps critical guardrails there as well as in `AGENTS.md`; it does not depend on JetBrains automatically loading `AGENTS.md`.

Default profile: `bookdb-primary` without the `agent` tool. It can implement, test and manage GitHub through the connected tools. Optional `bookdb-delegating` is selected only after the capability check proves bounded child execution. Executor/reviewer profiles do not include the agent-invocation tool and explicitly prohibit recursive delegation. All profiles inherit the selected model; no cloud model is named. Tool aliases may be ignored by unsupported versions, so confirm effective permissions rather than assuming the YAML is enforcement [R12].

GitHub's customization matrix currently does not list JetBrains hooks as supported [R03]. Therefore this package does not promise that repository hooks enforce concurrency or block Git worktree commands. Safe default is sequential execution. A hooks-based agent governor is not a prerequisite.

## One-time capability acceptance

Record actual versions/results in the S0 issue (no new permanent inventory document required):

| Probe | Pass result |
| --- | --- |
| Local model selection | Copilot uses the intended network model; no silent provider fallback |
| File read/edit | Reads a named test file and makes a reversible scoped edit in the canonical root |
| Terminal/run config | Executes a bounded test, reports exit code and can distinguish still-running from success |
| GoLand/WebStorm MCP | Resolves a known symbol or reports a real inspection in the intended project |
| DataGrip or psql | Reads current DB/schema using read-only credentials; no accidental production writes |
| GitHub | Lists current branch/issue/PR; validates permission for required operations |
| Playwright | Opens the local app and verifies a real element/state |
| Optional delegation | One child performs read-only work, returns, cannot recursively spawn; a second child is tried only if global capacity is controlled |

If delegation fails, remain on the sequential profile and continue application work. If a nonessential MCP fails, use an existing deterministic tool. Do not repeatedly reinstall the whole IDE stack, reconstruct the repository or change inference settings to repair a single missing tool.

## Operating boundary

IDE-driven autonomy requires the IDE/client session and laptop to remain available. MCP access to another IDE does not create a persistent background agent service. The user's current request does not require a daemon or webhook agent harness. GitHub workflows can perform deterministic CI/maintenance without invoking the local model; they must not send uncontrolled parallel jobs to 1cat-vllm.
