# deployment/compose/NOTES.md
#
# Development Compose notes
#
## Pinned images

The core image tags used in `compose.yaml` are major-line-pinned (not
`:latest`) so that a fresh `docker compose pull` produces predictable,
reviewable behavior. If you need to update a pin, open a dedicated PR to
change the tag and record the reason in the PR description.

| Service       | Image                            | Tag         | Notes |
| ------------- | -------------------------------- | ----------- | ----- |
| PostgreSQL    | `postgres`                       | `18`        | Canonical store; `bookdb` DB / user. |
| PgBouncer     | `edoburu/pgbouncer`              | `2.7.0`     | Transaction-mode pooler. |
| NATS          | `nats`                           | `2.12-alpine3.20` | JetStream enabled; monitoring on 8222 (dev only). |
| Valkey        | `valkey/valkey`                  | `9.0`       | Cache / rate-limit / ephemeral coordination. |
| OpenSearch    | `opensearchproject/opensearch`   | `3.0.0`     | Search projection; security plugin disabled in dev. |
| SeaweedFS     | `chrislusf/seaweedfs`            | `4.22`      | S3-compatible reference object storage; master/volume/filer/s3. |

Update these pins on release; see `project-management/RELEASE_CHECKLIST.md`
for the image-scan / SBOM gate.

## Not HA

Multiple containers on one host are NOT a distinct-failure-domain HA
deployment. This compose is a one-host development environment only. For a
true HA reference topology (multi-host Postgres + Patroni + etcd, multi-node
OpenSearch, multi-NATS JetStream mirrored, multi-node Seaweed volume set),
see `deployment/HA_REFERENCE_TOPOLOGY.md`.

## Local credentials

All credentials in this compose are **development placeholders** suitable for
the canonical dev profile:

- `postgres://bookdb:bookdb@postgres:5432/bookdb`
- `nats://nats:4222`
- `redis://valkey:6379/0`
- `http://opensearch:9200` (security plugin disabled)
- `http://seaweed-s3:8333` with `bookdb` / `bookdb-dev-secret-do-not-use-in-prod`

Production NEVER uses these. Use `BOOKDB_SECRET_*` env references or a
secrets manager. CI never reuses dev secrets against shared infrastructure.
