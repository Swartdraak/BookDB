# GitHub repository operations

GitHub Issues are the authoritative task database. This package uses native features for a specific purpose; it does not replicate them in `.agent-state`. `stages.json` is a versioned planning/acceptance seed, not a live status ledger.

## Feature ownership

| Feature | Purpose and management rule |
| --- | --- |
| Issues | Bugs, implementation slices, source/quality work and human acceptance gates; acceptance criteria and evidence stay with the issue |
| Milestones | S0–S8 delivery stages and later releases; close only when implementation and required human gates pass |
| Projects | Board/table/roadmap views of the same issues/PRs; no duplicate draft tasks for work already represented by an issue |
| Labels | Type, area, priority and one current status; `gate:human` marks human acceptance tasks |
| Branches | `feat/<issue>-<slug>`, `fix/<issue>-<slug>`, `chore/<issue>-<slug>` from current `main`; one coherent PR per slice |
| Pull requests | Problem, behavior, issue link, test evidence, migration/API impact and review; squash merge by default |
| Actions | Reproducible build/test/security/docs checks, trusted maintenance and artifact publication |
| Rulesets/protection | Require PRs, current checks, resolved review discussions, block force push/deletion of main |
| CODEOWNERS | Real maintainer ownership, particularly security/API/schema; do not invent GitHub users for AI roles |
| Wiki | Published navigation and operator/developer entry pages linking to versioned docs; generated from approved main, not a second spec |
| Discussions | Questions/design exploration when enabled; convert actionable accepted work into Issues |
| Releases/tags | Immutable candidate/stable versions, notes, upgrade instructions, artifact digests and supported configurations |
| Packages | Tested OCI image and eligible SDK packages; publish only in the release pipeline |
| Security/Dependabot | Security policy, private reporting where available, alerts and focused dependency PRs |
| Environments | Named test/RC/production boundaries and secrets; production protections must not be bypassed |

## Setup supplied by this package

After the local adoption branch exists, run:

```bash
python3 scripts/project/github_setup.py --repo Swartdraak/BookDB
python3 scripts/project/github_setup.py --repo Swartdraak/BookDB --apply
```

The first command is a read-only preview; the second creates missing package labels, S0–S8 milestones and 28 seeded issues. Matching uses stable issue markers, not title guesses. It does not rewrite existing issue bodies, reopen closed issues or replace unrelated labels. It enables relevant repository features without changing repository visibility. `--project` additionally creates/reuses the named Project, links it and adds seed issues. Project permissions may require separate account authorization; missing scopes must be reported honestly. Re-running is intended to converge without duplicate issues/milestones/Project items.

```bash
python3 scripts/project/github_setup.py --repo Swartdraak/BookDB --project
python3 scripts/project/github_setup.py --repo Swartdraak/BookDB --project --apply
```

The script does not set branch protection automatically: first inspect current rules and actual successful check names. It also does not certify human tests, publish releases or close milestones. Those are lifecycle actions with evidence, not installation side effects.

## Issue lifecycle

Statuses: `status:backlog`, `status:ready`, `status:in-progress`, `status:review`, `status:human-testing`, `status:blocked`; closed issue state represents completion. Maintain exactly one status label on open work. Remove obsolete status labels when moving state. Priority labels `priority:p0` through `priority:p3`; p0 is urgent correctness/security/availability. Type labels: feature, bug, maintenance, documentation, data-quality, source, acceptance. Area labels: backend, database, ingestion, webui, api, security, operations, integration.

An issue becomes ready when its outcome, prerequisites and meaningful test path are clear. Its body references requirement IDs and stage. The seeded issue body includes the stage tests; refine only the next ready slice into precise assertions. Do not generate dozens of tiny paperwork issues.

For a defect: reproduce, identify affected version/data scope, add a regression test, fix, record validation and release inclusion. For source/data-quality issues: include entity/source IDs and evidence without private user content. For blocked work: name the exact dependency and the next action; continue other ready work within the active stage. Do not close blocked work as completed.

Human acceptance issues have `gate:human` and remain open until a maintainer records pass/fail for the exact candidate. An agent can add `status:human-testing`, prepare evidence and fix defects. It cannot supply the human result.

## Project configuration

Use Project title `BookDB delivery`. Native Status (Todo/In Progress/Done) is a broad view; issue status labels remain the detailed state. Add single-select Priority (P0–P3), Stage (S0–S8), and Gate (Automated/Human). Add an iteration/date field only after useful delivery cadence data exists. The supplied setup creates missing fields; agents set values as they refine issues.

Create these saved views once: **Active stage board** (group by Status, filter active milestone), **Backlog** (table grouped by milestone), **Human testing** (filter `label:gate:human is:open`), **Defects** (filter bug/data-quality), **Release roadmap** (milestone/target dates as established). Use native auto-add/status workflows where the account supports them. Otherwise the primary agent updates the Project after each issue/PR transition via `gh project`/GraphQL. A scheduled reconciliation workflow is optional after credentials exist; do not pretend `GITHUB_TOKEN` automatically has user-Project authority [R16–R17].

## Pull request and branch policy

One canonical checkout. Switching branches affects every IDE viewing it, so only the primary switches/commits after all writers stop. Use scoped `git add <paths>` and inspect `git diff --cached`. Keep unrelated user edits intact. No direct commits to main. No force-push to shared branches. Do not create per-agent branches or worktrees.

Before merge: verify linked issue/criteria, review final diff, run relevant tests, ensure required CI succeeded on the current candidate, resolve discussions and record review findings. A separate agent review can assist; same-account automation is not a second GitHub approving reviewer. For this single-owner autonomous workflow, do not configure an impossible mandatory second-human approval on every PR. The stage gates provide human acceptance. If existing rules require a reviewer, honor them and record the access/capability need; do not bypass rules to make autonomy appear successful.

Use `gh pr merge <number> --squash --delete-branch` only after merge readiness is actually established. Do not use `--admin` as a routine workaround. After merge, fetch/prune safely and fast-forward local main once the checkout is clean; do not delete unmerged work or use blanket cleanup scripts.

## Required checks and adoption sequencing

The replacement CI supplies Go checks, WebUI checks, Application image, and Project docs. Preserve existing secret/dependency/image security checks while fixing their real deficiencies. S0 adds actual application-build testing because the old `go build ./...` missed the absent entry point. Later stage integration/contract/E2E jobs are added as capabilities land; do not configure nonexistent check names as required.

Inspect protections/rulesets before mutation. If the old `validate-agent-kit` or `validate-task-artifacts` contexts are required, use the owner's authorized policy replacement to exchange only those obsolete contexts for the new passing checks as part of adoption. Do not disable the entire ruleset or remove security checks. The observed baseline reports main unprotected, but the live state must be rechecked. Configure PR requirement, no force push/deletion, resolved conversations, up-to-date successful checks and squash policy. Preserve stronger unrelated protections already in place. Save the before/after settings in the adoption issue.

## CI cost and trust boundaries

Use standard GitHub-hosted Linux runners for normal public-repo PR checks [R15]. Avoid running untrusted fork code on persistent homelab/self-hosted runners. Large data/HA tests run only on trusted reviewed code in disposable test environments. Workflows handling untrusted issue titles/bodies must pass data as JSON/environment input, never interpolate it into shell code. Do not use `pull_request_target` to execute PR code with secrets.

Set timeouts/concurrency cancellation on superseded CI runs. Keep routine artifacts about 7 days, failed diagnostics 14 days, release artifacts with releases. Cache dependencies, not datasets or secrets. Review billing/storage monthly and keep spending controls at the owner's chosen limit. Pin external actions to reviewed commit SHAs before GA and manage updates with Dependabot; maintained major tags in the initial templates are adoption conveniences, not immutable release pins.

## Wiki and ongoing management

`python3 scripts/project/wiki_sync.py --repo Swartdraak/BookDB` previews generated Wiki pages; `--apply` synchronizes only BookDB-generated navigation pages from committed main. It preserves unrelated Wiki pages. GitHub may require the first Wiki page to be created through the UI before its Git repository exists; if so, one-time account interaction is necessary [R21]. The Wiki is not allowed to delay application work.

The optional manual workflow runs Wiki sync only with configured credentials; no unavailable secret is treated as success. Primary agent duties after each merge: reconcile linked issues/Project, update affected canonical docs, synchronize Wiki if public navigation changed, check current milestone readiness and choose the next ready task. Weekly during maintenance: triage new issues/discussions, inspect dependency/security alerts, check source freshness and CI costs. Do not automatically close old unresolved bugs or create a new governor service for these duties.
