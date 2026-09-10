# Test strategy, commands and human acceptance

Status: the package provides test contracts and an acceptance dispatcher. It does not claim to have run BookDB in the user's environment. Future acceptance runners must be implemented with each stage, not supplied as fake passing scaffolds.

## Commands: existing versus required

| Command | Status at inspected baseline | Meaning |
| --- | --- | --- |
| `make fmt-check`, `make lint`, `make test`, `make build`, `make openapi-validate` | Exist | Existing Go checks; build checks packages and does not prove a runnable application |
| `npm --prefix web ci` | Exists through npm | Install lockfile dependencies |
| `npm --prefix web run format`, `typecheck`, `lint`, `test`, `build` | Scripts exist | Frontend checks; `format` checks rather than writes |
| `make compose-app-build` | Target exists; referenced `cmd/bookdb` absent | Must be repaired and run during S0 |
| `make compose-app-up` | Target exists | Must be verified against real runtime/config after S0 repair |
| `make compose-down` | Exists but uses `down -v` | S0 changes ordinary shutdown to preserve volumes; never use baseline target against valuable data |
| `python3 scripts/project/check_docs.py` | Supplied by this package | Check active local documentation links; no application test substitute |
| `python3 scripts/project/acceptance.py S0 --describe` | Supplied | Show stage criteria without implying tests passed |
| `python3 scripts/project/acceptance.py S0` | Supplied dispatcher, stage runner must be implemented | Execute real stage acceptance; fails if runner missing |
| `scripts/acceptance/s0.sh` … `s8.sh` | Required stage deliverables | Orchestrate meaningful CLI/API/database/browser checks with artifacts |

The existing module path is `github.com/bookdb/bookdb`; do not casually rename it while repairing build plumbing. S0 determines why the entry point/config are missing and restores a working runtime consistent with package code. The new CI explicitly builds `./cmd/bookdb` and the application image so a library-only build cannot mask this gap.

## Acceptance runner contract

Each stage runner must support its documented environment variables, fail nonzero on any required assertion, and write a report under `.local/acceptance/<stage>/<timestamp>/` (Git-ignored). Minimum report: commit SHA; dirty/clean state; dependency/image versions; dataset/snapshot hash; commands; pass/fail/skipped counts; relevant HTTP/SQL results; artifact paths; elapsed time; cleanup performed; known blockers. `NOT IMPLEMENTED`, skipped required checks and unavailable required services do not count as pass.

Runners start only disposable services/databases or reuse an explicitly named test environment. Avoid fixed `container_name` values and global Compose project names in test profiles; use a unique `COMPOSE_PROJECT_NAME` and resource labels. Cleanup deletes only resources the runner created. Never run `docker system prune`, broad `rm -rf`, or volume deletion against the developer's normal catalog. A reset command must name the disposable project/database and require an explicit test-mode guard.

Network-independent tests use small local fixtures. Real-source smoke/import tests use recorded, eligible source snapshots and report their identity. PR CI must not download a global catalog on every run. Bulk/HA acceptance runs on allocated test infrastructure with permission and isolation, not on an untrusted PR's access to the homelab.

## Test layers

| Layer | Important assertions | Typical runner |
| --- | --- | --- |
| Unit/property | Normalization, ISBN checksums, date precision, role mapping, deterministic rules, cursor validation | Go tests; Vitest |
| Database integration | Real migrations, constraints, transactions, concurrency, inbox/outbox and populated upgrades | Go tests against disposable PostgreSQL |
| Service integration | NATS redelivery, OpenSearch revision/order/rebuild, Valkey distributed quotas, S3 object/persistence behavior | Real containers; no universal mock adapter |
| API contract | Every operation, error status, auth/scope, pagination, nullability and version behavior | OpenAPI validation plus request/response tests |
| Browser E2E | Search→edition, login, key rotation, private proposal→approval, source job controls | Playwright tests committed to repository |
| Data quality | Hard-negative resolution corpus, precision/recall, source order/replay invariance, accounting | Versioned fixtures and labelled benchmark |
| Operational | Fresh install, upgrade, interruption, backup/restore, failover and capacity | Stage scripts on isolated infrastructure |
| Human acceptance | Bibliographic judgment, usability, screen reader, actual OIDC, integrated consumer operation | Written stage checklist; human-recorded result |

Test outcomes must validate behavior rather than repeat implementation structure. For example, a test that a “requires_api_key” config flag is true does not prove unauthorized requests are denied. Tests using a mock migrator do not prove PostgreSQL schema validity. Reviewing code is not equivalent to a runtime assertion.

## Mandatory regression corpus

Maintain fixed synthetic cases with expected identity graphs: same title/different author; same name/different people; same work in English/Korean; same-language two narrations; abridged/unabridged; print/ebook; ISBN-10 and ISBN-13 normalization; malformed/shared ISBN; no identifier; pseudonym; two translators; decimal series order; omnibus with two works; bilingual edition; publisher versus producer; unknown duration/date precision; source merge/delete; stale/duplicate events; unapproved proposal with private asset.

Synthetic fixture text is clearly marked test data. Do not present it as real source metadata. Keep a separate small, redistribution-eligible real-source subset for connector mappings. Golden expected results are reviewed before algorithm tuning; changing expected values requires an explanation in the PR.

## Change-feed correctness trap

A sequence allocated inside transactions does not automatically produce commit order. If transaction A gets sequence 10, transaction B gets 11 and commits first, a client advancing to 11 could skip A when it commits later. Use a serialized publication mechanism or a post-commit durable feed sequencer with a correctly maintained watermark. Tests must deliberately reproduce this interleaving. The feed cursor advances only across committed, eligible events; private proposal events are excluded.

## Human testing handoff

At S2/S5/S7/S8, prepare one issue comment using this template:

```text
Candidate: exact commit and image digest(s)
Stage and linked implementation PRs:
Environment URL(s) and how to obtain test credentials locally:
Setup: exact fresh-install/upgrade commands and fixture/snapshot
Automated evidence: CI run/report links and known failures
Steps: numbered actions with an expected visible/API/DB result each
Pass criteria: exact stage checklist
Reset/cleanup: named disposable resources only
Rollback/recovery: verified command or documented procedure
Human result: NOT RUN (reserved for the human)
```

The agent stops before advancing past the gate. A maintainer comment with the exact candidate and pass/fail result is the authoritative acceptance record; a label or AI-generated screenshot is not human approval. At S8, GA promotion must use the accepted artifacts, not rebuild an untested moving `main`.

## Verification economy

Run the smallest relevant checks while editing; run the required PR gates on the final candidate. Repeat broader suites only after relevant changes or a failed gate. Avoid repeated full repository scans, all-source downloads or GPU stress tests on every task. Do not write tests for prose word counts, arbitrary role counts, or the existence of governance paperwork.
