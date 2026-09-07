# User Contribution, Administration, and Publication Workflow

## Binding owner decision

Users may propose additions and edits. **Nothing derived from a user proposal becomes public canonical metadata until an Administrator approves it.**

## Roles

### User
May browse and create proposals if enabled.

### Contributor
Optional role with improved proposal tools/history; same publication restriction.

### Moderator
Optional triage role:
- validate formatting;
- flag duplicates;
- request clarification;
- recommend approve/reject;
- combine duplicate proposals.

A Moderator **cannot publish user-supplied catalog data**.

### Administrator
Only role allowed to approve publication of user-provided claims/entities/assets.

### Service
Machine API account; no implicit curation privilege.

## Proposal states

```text
DRAFT
SUBMITTED
VALIDATING
NEEDS_INFO
READY_FOR_ADMIN
APPROVED
REJECTED
WITHDRAWN
SUPERSEDED
```

Only `APPROVED` can create publishable user-origin claims.

## Proposal types

- new Work
- new Expression
- new Edition
- new Person/Organization
- new Series
- field correction
- relationship correction
- series order
- external identifier
- cover/person image
- merge suggestion
- split suggestion

## Publication transaction

On Administrator approval:
1. validate current proposal revision;
2. re-run duplicate/identity checks;
3. create CURATOR/USER_APPROVED claims with contributor attribution policy;
4. run reconciliation;
5. write audit event;
6. write transactional outbox;
7. return approval result;
8. search update follows asynchronously.

## Conflicts after submission

If upstream/source data changes while proposal waits:
- proposal is not silently reinterpreted;
- Administrator sees “base changed” warning;
- evidence comparison refreshes;
- approval may require reconfirmation.

## Administrator evidence view

Must display:
- current canonical value;
- proposed value;
- source claims;
- provenance;
- entity match confidence;
- contradictions;
- duplicate candidates;
- submitter explanation;
- rights information for assets.

## Abuse controls

- rate limits;
- account age/reputation optional;
- proposal throttles;
- HTML/script sanitization;
- links treated as untrusted;
- audit trail;
- ban/suspension controls;
- no direct user-uploaded executable content.

## Public attribution

Whether contributor username is public is instance-configurable. Audit identity is always retained privately for authorized administrators.
