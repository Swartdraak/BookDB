# Escalation Policy

Agents MUST stop and escalate when:

## Architecture
- accepted ADR conflicts with task;
- new dependency/platform is required;
- public contract requires breaking change;
- event contract cannot remain backward compatible.

## Domain
- Work/Expression/Edition semantics unclear;
- identifier scope unclear;
- inheritance could publish incorrect metadata;
- merge/split semantics change.

## Source/legal
- source license/terms unclear;
- persistence rights uncertain;
- asset rights uncertain;
- source access restrictions changed.

## Security
- task requires secret disclosure;
- requested authorization bypass;
- threat changes trust boundary;
- critical/high vulnerability cannot be safely resolved within lease.

## Ownership
- required edit lies outside lease;
- another active lease owns path;
- shared-file collision detected.

## Quality
- reconciliation benchmark regresses beyond task threshold;
- migration cannot be safely tested;
- required tests are unavailable or non-deterministic.

## Escalation output

Must include:
- task ID;
- agent;
- blocker category;
- exact issue;
- affected requirement/ADR/path;
- what decision is needed;
- recommended recipient;
- safe current state.
