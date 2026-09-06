# BookDB Development MCP Server Specification

The MCP is a repository/development control plane for authorized agents.

## Read-only tools

- `docs.search`
- `adr.list/get`
- `source_policy.list/get`
- `source_schedule.list/get`
- `schema.describe`
- `migration.status`
- `nats.contracts.list/get`
- `openapi.inspect`
- `tests.last_results`
- `compose.status`
- `service.health`
- `logs.query`
- `metrics.query`
- `github.issue/pr/status`

## Controlled development actions

- `tests.run`
- `lint.run`
- `compose.up/down`
- `migration.apply` (dev/test only)
- `source.fixture.validate`
- `source.sync` (fixture/dev instance only by default)
- `openapi.validate`
- `search.reindex` (dev/test)
- `ha.failover_test` (lab)
- GitHub branch/PR/comment through authorized integration

## Hard prohibitions

- production destructive DB SQL;
- exposing stored connector/OIDC/API secrets;
- changing blocked source policy without repository review;
- approving user metadata as Administrator autonomously;
- bypassing Administrator publication gate;
- auto-merging security-sensitive changes;
- claiming HA from single-host replica count.
