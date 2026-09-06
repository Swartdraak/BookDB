# Agent Orchestration Matrix

| Work type | Lead agent | Required reviewers |
|---|---|---|
| New feature | orchestrator / relevant engineer | QA + domain/security as applicable |
| New metadata source | source-governance + ingestion | scheduler, QA, security |
| Sync schedule change | source-scheduler | source-governance, observability |
| Data model | bibliographic-domain | architect, database, API |
| Identity rule | identity-reconciliation | bibliographic-domain, QA |
| DB migration | database | postgres-ha, QA |
| HA topology | postgres-ha / distributed-systems | devops, security |
| NATS event | distributed-systems | relevant producer/consumer, QA |
| Search mapping | search | API, QA |
| Asset storage | object-storage | source-governance, security |
| Auth/OIDC | auth-identity | security, QA |
| User contribution flow | auth-identity + domain | QA, API/WebUI |
| API change | backend-api | QA, SDK compatibility |
| Release | devops-release | security, QA, PM |
| Incident | incident-maintainer | affected domain agent |
| GitHub automation | github-governor | security/devops where writes occur |
