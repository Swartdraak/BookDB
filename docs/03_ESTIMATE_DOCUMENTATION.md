# Estimate Documentation

The final owner decisions increase infrastructure, testing, and operational scope versus the earlier single-node-first estimate.

## Planning estimate

| Workstream | Engineering person-weeks |
|---|---:|
| Repository/tooling/licenses/CI | 4–6 |
| Domain schema/migrations | 6–10 |
| PostgreSQL HA/PITR/failover | 5–8 |
| NATS distributed processing/outbox | 5–8 |
| Source policy/scheduler | 4–6 |
| Connector SDK + first sources | 10–15 |
| Identity/reconciliation engine | 12–18 |
| OpenSearch/search relevance | 6–10 |
| S3 assets/image pipeline | 4–7 |
| Native API/OpenAPI/SDK | 7–11 |
| Local auth/OIDC/RBAC | 4–7 |
| Proposal/Admin publication | 5–8 |
| Public/admin WebUI | 10–15 |
| Additional metadata connectors | 8–14 |
| Observability/performance | 5–8 |
| Security/hardening | 5–8 |
| Deployment/backup/DR/HA testing | 6–10 |
| PVR integrations | 5–9 |
| Documentation/release | 4–7 |

**Planning envelope:** approximately **110–177 engineering person-weeks**.

This is a forecasting range, not a commitment or calendar duration.

## Largest uncertainty drivers

- identity-resolution precision;
- source licensing/availability;
- data volume during full bootstrap;
- search relevance across languages;
- migration performance;
- administrator moderation UX;
- number of production-grade source connectors required for v1;
- reference HA environments.

## Re-estimation gates

Re-estimate after:
1. M2 distributed platform;
2. first full Open Library/Wikidata bootstrap;
3. gold-corpus identity benchmark;
4. OpenSearch relevance/load benchmark;
5. WebUI/admin beta.
