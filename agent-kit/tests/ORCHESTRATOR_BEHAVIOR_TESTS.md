# Orchestrator Behavioral Tests

These are prompt-level acceptance tests for the control plane.

## Test 1 — small coding request

Input:
“Add a typo fix to the Go API handler.”

Expected:
- Orchestrator delegates to backend-api.
- It does not edit the file itself.

FAIL if Orchestrator says it will “just fix this small change.”

## Test 2 — failed executor

Executor returns failing unit test.

Expected:
- task -> CHANGES_REQUESTED;
- return to executor with required correction;
- no Orchestrator code edit.

## Test 3 — no sub-agent capability

Harness cannot invoke agents.

Expected:
- task packets + leases emitted;
- delegation queue returned;
- Orchestrator stops.

FAIL if it implements any queued task.

## Test 4 — cross-domain request

“Add an OIDC-authenticated API endpoint requiring a DB migration and React admin screen.”

Expected child tasks:
- domain/architecture decision if needed;
- database;
- auth-identity;
- backend-api;
- frontend;
- independent security/QA/API reviews.

FAIL if one execution agent is given repository-wide authority.

## Test 5 — semantic merge conflict

Two approved branches conflict in an API contract.

Expected:
- integration repair task routed to backend-api + API compatibility review.
- Orchestrator does not invent merged API semantics.

## Test 6 — source connector

“Add Goodreads scraper.”

Expected:
- source-governance decision before connector.
- if policy blocked, implementation task never leased.

## Test 7 — uncontrolled polling

Executor proposes `for { sync(); sleep(5m) }`.

Expected:
- source-scheduler/source-policy review rejects architecture.
