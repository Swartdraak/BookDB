# GitHub Repository Blueprint

## Branch policy

`main` protected:
- no direct pushes;
- required PR;
- required status checks;
- conversation resolution;
- CODEOWNERS where applicable;
- signed release tags.

Short-lived branches:
- `feat/<issue>-...`
- `fix/<issue>-...`
- `data/<issue>-...`
- `source/<issue>-...`
- `docs/<issue>-...`
- `chore/<issue>-...`

## Required checks

- Go fmt/vet/lint
- Go unit
- Go integration
- fuzz smoke
- frontend lint/type/test
- Playwright E2E
- migration fresh
- migration upgrade
- event schema compatibility
- connector contract
- OpenAPI lint/compat
- source-policy validation
- data-quality corpus (when changed)
- dependency/license scan
- secret scan
- container build
- SBOM
- image vulnerability scan

## GitHub Projects fields

- Status
- Milestone
- Epic
- Priority
- Area
- Owner
- Risk
- Target release
- Blocked by
- Data migration
- API change
- Source policy review

## Labels

Types:
`type:feature`, `type:bug`, `type:data-quality`, `type:source`, `type:security`, `type:docs`, `type:architecture`.

Areas:
`area:catalog`, `area:identity`, `area:reconcile`, `area:ingest`, `area:scheduler`, `area:db`, `area:nats`, `area:search`, `area:assets`, `area:api`, `area:web`, `area:auth`, `area:ops`.

Priority:
`P0`–`P3`.

## Automation

Safe automation:
- label/triage;
- stale report (not blind close);
- dependency PRs;
- generated SDK/OpenAPI check;
- release notes;
- CI result comments.

Dangerous automation requires explicit human gate:
- source-policy status changes;
- user proposal approvals;
- production migration;
- release publication;
- security-sensitive merge.
