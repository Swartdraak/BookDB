# ADR 0011 — Preserve implementation and reset delivery governance

Date: 2026-09-10. Status: adopted project direction from the owner's current instructions; repository adoption occurs through its PR.

## Context

The previous agent system mandated non-implementing orchestration, task packets, path leases and repeated certification. PR #8 repaired excessive worktrees, and PR #13 added schema/auth foundations, but the new owner direction requires ordinary branch/commit/PR delivery, at most two children on local inference, sequential fallback and working stages.

## Decision

Preserve application code, migrations, tests and Git history. Retire the active agent-kit/agent-state control plane and duplicate instructions. Use GitHub Issues/Milestones/Projects for state, one canonical checkout, one writer and optional bounded delegation. Primary agents can implement. Human gates are S2/S5/S7/S8. No automatic worktrees or full application copies.

Retain ADRs 0001–0009 on licensing, modular Go architecture, PostgreSQL, NATS, OpenSearch, S3, administrator moderation, scheduled sync and local/OIDC auth. Interpret “HA from day one” as architecture boundaries and durable primitives from the start, with real multi-host proof in S6 before GA. Supersede the old platform-first M2–M11 sequencing with behavior-based S0–S8. Actual dependency manifests describe installed versions; major upgrades are focused tested changes, consistent with dependency-tooling ADR 0010.

Amend the minimal M1 identity model using forward migrations: expression title/language is not unique identity, ISBN assertions can conflict, editions can embody multiple expressions, and published identity history cannot be erased by cascades. These changes implement the stated multi-format end state; they do not discard existing catalog data.

## Alternatives and consequences

Keeping and repairing every lease validator would preserve the failure mode the owner rejected. A fresh rewrite would lose useful code/history. A new agent governor would add another system before product delivery. The chosen approach reduces coordination artifacts but requires honest final-diff review, meaningful tests and actual human acceptance. Markdown alone does not enforce backend concurrency; sequential mode remains the default when tool controls are uncertain.

Active authority: [handbook](../BOOKDB_PROJECT.md), [migration](../docs/bookdb/14-migration.md), [agent operations](../docs/bookdb/10-agent-operation.md).
