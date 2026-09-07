# M0 Governance Recovery and Certification

Restore canonical Agent Kit v3 paths, certify the existing M0 implementation through delegated review, repair governance/security defects, and integrate the repository foundation to main.

Current control-plane state:
- M0 milestone: https://github.com/Swartdraak/BookDB/milestones/1
- Parent issue: https://github.com/Swartdraak/BookDB/issues/2
- Stage A PR merged: https://github.com/Swartdraak/BookDB/pull/1
- Stage B PR merged: https://github.com/Swartdraak/BookDB/pull/4

## GitHub state
- Repository: Swartdraak/BookDB
- Target branch: `main`
- Stage A branch: `fix/m0-governance-certification`
- Stage B branch: `feat/m0-repository-foundation`
- Pull requests: https://github.com/Swartdraak/BookDB/pull/1 and https://github.com/Swartdraak/BookDB/pull/4
- Parent issue: https://github.com/Swartdraak/BookDB/issues/2
- Milestone: https://github.com/Swartdraak/BookDB/milestones/1
- Devcontainer remediation issue: https://github.com/Swartdraak/BookDB/issues/3

## Scope
- GitHub governance metadata for M0 only.
- No application code changes.
- Durable control-plane tracking under `.agent-state` only.

## Acceptance criteria
- The M0 milestone exists and matches the repository roadmap terminology.
- The parent M0 issue links to the recovery PR and captures certification criteria.
- The recovery PR description reflects the actual M0 recovery/certification scope.
- A durable local record exists under `.agent-state`.
