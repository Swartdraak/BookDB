# Risk Register

| Risk | Impact | Mitigation |
|---|---|---|
| False identity merge | Critical data corruption | high thresholds, contradiction gates, split history, gold corpus |
| Source terms change | legal/operational | versioned source policy + scheduled review + kill switch |
| Unbounded source sync | provider ban/resource waste | schedule/manual only, budgets, jitter, conditional requests |
| Duplicate distributed job | inconsistent side effect | idempotency + DB constraints + at-least-once-safe workers |
| DB primary split brain | catastrophic | Patroni/DCS/fencing/failover drills |
| Queue quorum loss | processing outage | 3/5 JetStream nodes, replicated streams |
| Search cluster loss | search outage | replicated indexes + rebuild from PostgreSQL |
| Object-store loss | missing images | replicated S3, checksums, backups |
| Cache loss | latency spike | cache-as-optional design |
| Admin approves bad user data | quality issue | evidence UI, audit, reversible claims/merge/split |
| Catalog exceeds initial DB layout | performance | partition large history tables, benchmark, capacity tests |
| Dependency licensing conflict | release blocker | license scan + dependency policy |
| MinIO ecosystem change | storage risk | S3 abstraction; SeaweedFS default |
