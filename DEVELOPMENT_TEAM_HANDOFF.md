# Development Team Handoff

## Engineering invariant

Source data is evidence. BookDB publishes canonical truth only through the reconciliation/publication layer.

## Initial implementation order

1. repository/toolchains/CI;
2. core IDs/domain packages;
3. PostgreSQL schema/migrations;
4. NATS event contracts/outbox;
5. source policy/scheduler;
6. connector SDK;
7. raw source/claims;
8. identity engine;
9. reconciliation;
10. OpenSearch projection;
11. S3 assets;
12. API;
13. local/OIDC auth;
14. proposal/Admin publication;
15. WebUI;
16. sources/integrations;
17. HA/performance hardening.

## Do not shortcut

- no direct source -> canonical writes;
- no in-memory-only queue;
- no user edit direct publication;
- no filesystem-specific production asset interface;
- no OpenSearch as source of truth;
- no uncontrolled polling;
- no hard-coded provider IDs as BookDB IDs.

## Agent usage

Load `agent-kit/AGENTS.md`, then select lead/review roles from `agent-kit/AGENT_ORCHESTRATION_MATRIX.md`.
