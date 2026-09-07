# M1 Lifecycle Kickoff

Begin M1 immediately after M0 certification on main by creating delegated planning work and explicit acceptance criteria for the first M1 execution wave.

## GitHub state
- Repository: Swartdraak/BookDB
- Baseline branch: `main`
- Kickoff branch: `feat/m1-lifecycle-kickoff`
- Upstream M0 completion PR: https://github.com/Swartdraak/BookDB/pull/4

## Scope
- Control-plane planning artifacts under `.agent-state`.
- Delegation-ready M1 task packet(s).
- No application implementation changes.

## Acceptance criteria
- M0 closure is recorded in `.agent-state` with completed task/lease status for Stage A and Stage B integration repairs.
- At least one M1 planning task exists in READY_TO_LEASE state with clear in-scope/out-of-scope boundaries.
- A durable M1 kickoff ledger exists for onward tracking.
