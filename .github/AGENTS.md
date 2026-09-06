# Global Agent Contract

## Mandatory initial context
- `README.md`
- `docs/09_REQUIREMENTS_DOCUMENTATION.md`
- `docs/10_ARCHITECTURE_AND_DESIGN_DOCUMENTATION.md`
- `docs/15_SOURCE_POLICY_AND_RESEARCH.md`
- `docs/20_DISTRIBUTED_PROCESSING_AND_HA.md`
- relevant ADRs

## Non-negotiables

1. Personal PVR/library data is never metadata input.
2. Technical access is not permission to ingest/persist.
3. User-provided catalog data is never public before Administrator approval.
4. Connectors run only by schedule/manual/event/bootstrap/replay — no uncontrolled constant polling.
5. Source records/claims never bypass reconciliation into canonical tables.
6. Unknown facts stay unknown.
7. PostgreSQL is canonical.
8. OpenSearch is rebuildable.
9. Valkey is cache/ephemeral state, not durable job truth.
10. NATS workers are idempotent and at-least-once safe.
11. Public canonical changes use transactional outbox/event publication.
12. Released migrations are immutable.
13. Merge/split uses domain operations, not destructive SQL.
14. Reconciliation changes require gold-corpus diffs.
15. Secrets never enter Git/logs/support bundles.
16. HA claims require distinct failure domains, not merely multiple containers on one host.

## Completion report

Every agent task reports:
- issue/ADR;
- files changed;
- tests;
- migrations;
- API impact;
- data-quality impact;
- source-policy impact;
- distributed/HA impact;
- security impact;
- documentation;
- remaining risk.
