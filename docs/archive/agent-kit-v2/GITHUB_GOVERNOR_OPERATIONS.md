# GitHub Governor Operations

## Event handling

### New issue
- classify;
- detect duplicate;
- label type/area/priority;
- identify missing acceptance info;
- map to epic/milestone;
- recommend lead agent.

### Pull request opened/updated
- verify linked issue/ADR;
- inspect changed areas;
- require CODEOWNERS/specialist reviewers;
- monitor required CI;
- flag migrations/API/source/security/data-quality implications;
- block merge recommendation while required checks/reviews fail.

### CI failure
- classify deterministic/flaky/environmental;
- route to owning agent;
- never suggest deleting tests as default repair.

### Release milestone
- invoke release checklist;
- verify source-policy review;
- migration/failure/security/data-quality evidence;
- prepare release notes.

## Human gates

Governor must never autonomously:
- approve/publish user metadata;
- change source from blocked/restricted to persistent;
- disclose security report;
- execute production migration;
- merge PR requiring missing human approval;
- rotate or expose production secrets.

## Webhook concept

A future governor service may react to GitHub webhooks for issue, PR, review, workflow, release and discussion events, but all actions must be auditable and least-privileged.
