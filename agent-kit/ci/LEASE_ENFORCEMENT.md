# CI Lease Enforcement

## Goal

A PR produced by an autonomous execution agent should be verifiably limited to its task authority.

## Recommended PR metadata

PR includes:
- Task ID
- Lease ID
- path to committed TaskPacket
- path to committed AuthorityLease

## CI check

Run:

```bash
python tools/validate_contract.py task .agent-state/tasks/$TASK_ID.yaml
python tools/validate_contract.py lease .agent-state/leases/$LEASE_ID.yaml
python tools/check_lease_diff.py \
  --lease .agent-state/leases/$LEASE_ID.yaml \
  --base "$LEASE_BASE_COMMIT"
```

A lease violation fails CI.

## Human-created PRs

Human maintainers may use a separately defined `human-maintainer` process or a human-approved broad lease. Do not disable lease checking globally merely because some changes are manual.

## Shared generated files

If a generated lockfile/root manifest must change, explicitly include it in the lease. Do not add repository-wide wildcard authority.
