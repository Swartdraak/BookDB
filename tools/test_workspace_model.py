#!/usr/bin/env python3
"""Governance tests for the corrected Agent Kit workspace model.

Covers:
  - legacy schema 1.0 AuthorityLease validates
  - new canonical schema 1.1 lease validates
  - new isolated schema 1.1 lease validates
  - new read-only schema 1.1 lease validates
  - canonical mode does not require a registered Git worktree
  - isolated mode outside canonical repo passes
  - isolated mode nested inside canonical repo fails
  - read-only mode with write action fails
  - canonical workspace branch mismatch fails
  - isolated workspace branch mismatch fails
  - legacy M0 lease remains valid
  - executor path-lease checking still works
  - review/provenance behavior remains valid

Run: python3 tools/test_workspace_model.py
"""
import copy
import json
import subprocess
import sys
from pathlib import Path

try:
    import yaml
    import jsonschema
except Exception as e:  # pragma: no cover
    print("PyYAML and jsonschema required:", e, file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parents[1]
SCHEMA = json.loads(
    (ROOT / "agent-kit/contracts/AUTHORITY_LEASE.schema.json").read_text(encoding="utf-8")
)

failures = []
passes = []


def check(name, cond):
    if cond:
        passes.append(name)
    else:
        failures.append(name)


def validates(lease):
    try:
        jsonschema.validate(lease, SCHEMA)
        return True
    except jsonschema.ValidationError:
        return False


def base_lease_1_1(mode="CANONICAL", workspace="/path/to/BookDB", branch="feat/m1-x"):
    return {
        "schema_version": "1.1",
        "lease_id": "lease-M1-X-001",
        "task_id": "M1-X-001",
        "agent_id": "backend-api",
        "status": "ACTIVE",
        "base_commit": "abcdef1234567890",
        "branch": branch,
        "workspace_mode": mode,
        "workspace": workspace,
        "write_allow": ["internal/api/**"],
        "write_deny": ["migrations/**"],
        "allowed_actions": ["create", "modify", "commit"],
        "forbidden_actions": ["modify-outside-lease", "merge-to-main"],
        "required_reviewers": ["qa-reviewer"],
        "expires_at": None,
        "parameters": {},
    }


def base_lease_1_0():
    return {
        "schema_version": "1.0",
        "lease_id": "lease-M0-X-001",
        "task_id": "M0-X-001",
        "agent_id": "platform-devex",
        "status": "COMPLETED",
        "base_commit": "abcdef1234567890",
        "branch": "agent/M0-X-001",
        "worktree": ".agent-state/worktrees/M0-X-001",
        "write_allow": [".devcontainer/**"],
        "write_deny": ["internal/**"],
        "allowed_actions": ["create", "modify", "commit"],
        "forbidden_actions": ["modify-outside-lease", "merge-to-main"],
        "required_reviewers": ["qa-reviewer"],
        "expires_at": None,
        "parameters": {},
    }


# --- Schema validation ---
check("legacy 1.0 lease validates", validates(base_lease_1_0()))
check("canonical 1.1 lease validates", validates(base_lease_1_1("CANONICAL")))
check(
    "isolated 1.1 lease validates",
    validates(base_lease_1_1("ISOLATED", "/path/to/BookDB-worktrees/M1-X-001", "agent/M1-X-001")),
)
ro = base_lease_1_1("READ_ONLY")
ro["write_allow"] = []
check("read-only 1.1 lease validates", validates(ro))

# A 1.1 lease must NOT carry the legacy worktree field (additionalProperties=false).
bad = base_lease_1_1("CANONICAL")
bad["worktree"] = ".agent-state/worktrees/M1-X-001"
check("1.1 lease rejects legacy worktree field", not validates(bad))

# A 1.0 lease must NOT carry workspace_mode (additionalProperties=false).
bad10 = base_lease_1_0()
bad10["workspace_mode"] = "CANONICAL"
check("1.0 lease rejects workspace_mode field", not validates(bad10))

# --- Workspace-mode semantics (pre-edit assertion logic) ---
CANONICAL_REPO = "/path/to/BookDB"


def current_workspace_is_registered_worktree(workspace):
    """Simulate `git worktree list` membership for a path (test stub)."""
    # In a real harness this shells out to `git worktree list --porcelain`.
    # For the governance test we treat any path under the canonical repo as
    # NOT a valid isolated workspace, and any external path as registered.
    return not workspace.startswith(CANONICAL_REPO + "/")


def canonical_assertion(lease, current_workspace, current_branch):
    return (
        current_workspace == lease["workspace"]
        and current_branch == lease["branch"]
    )


def isolated_assertion(lease, current_workspace, current_branch):
    if current_workspace != lease["workspace"]:
        return False
    if current_branch != lease["branch"]:
        return False
    if current_workspace.startswith(CANONICAL_REPO + "/"):
        return False  # nested inside canonical repo
    if not current_workspace_is_registered_worktree(current_workspace):
        return False
    return True


def read_only_assertion(lease, current_workspace, current_branch, write_ops):
    if current_workspace != CANONICAL_REPO:
        return False
    if write_ops:
        return False
    return True


# canonical: no registered worktree required
c = base_lease_1_1("CANONICAL")
check(
    "canonical mode does not require a registered Git worktree",
    canonical_assertion(c, CANONICAL_REPO, c["branch"]),
)

# isolated outside canonical repo passes
iso = base_lease_1_1("ISOLATED", "/path/to/BookDB-worktrees/M1-X-001", "agent/M1-X-001")
check(
    "isolated mode outside canonical repo passes",
    isolated_assertion(iso, iso["workspace"], iso["branch"]),
)

# isolated nested inside canonical repo fails
iso_nested = base_lease_1_1(
    "ISOLATED", CANONICAL_REPO + "/.agent-state/worktrees/M1-X-001", "agent/M1-X-001"
)
check(
    "isolated mode nested inside canonical repo fails",
    not isolated_assertion(iso_nested, iso_nested["workspace"], iso_nested["branch"]),
)

# read-only with write action fails
ro2 = base_lease_1_1("READ_ONLY")
ro2["write_allow"] = []
check(
    "read-only mode with write action fails",
    not read_only_assertion(ro2, CANONICAL_REPO, ro2["branch"], write_ops=["modify"]),
)
check(
    "read-only mode without write action passes",
    read_only_assertion(ro2, CANONICAL_REPO, ro2["branch"], write_ops=[]),
)

# canonical workspace branch mismatch fails
check(
    "canonical workspace branch mismatch fails",
    not canonical_assertion(c, CANONICAL_REPO, "some-other-branch"),
)

# isolated workspace branch mismatch fails
check(
    "isolated workspace branch mismatch fails",
    not isolated_assertion(iso, iso["workspace"], "some-other-branch"),
)

# --- Legacy M0 lease remains valid (real historical artifact) ---
m0_lease_path = ROOT / ".agent-state/leases/lease-M0-RECOVERY-GOV-001.yaml"
if m0_lease_path.exists():
    m0 = yaml.safe_load(m0_lease_path.read_text(encoding="utf-8"))
    check("legacy M0 lease remains valid", validates(m0))
else:  # pragma: no cover
    check("legacy M0 lease remains valid (sample)", validates(base_lease_1_0()))

# --- Executor path-lease checking still works ---
sys.path.insert(0, str(ROOT / "tools"))
import importlib.util

spec = importlib.util.spec_from_file_location("check_lease_diff", ROOT / "tools/check_lease_diff.py")
# We only need the `matches` helper; import the module source without running argparse.
src = (ROOT / "tools/check_lease_diff.py").read_text(encoding="utf-8")
ns = {}
# Extract just the matches() function to avoid argparse side effects.
import re as _re

m = _re.search(r"def matches\(.*?\n(?=\nap = )", src, _re.S)
if m:
    from pathlib import PurePosixPath as _PP
    ns2 = {"PurePosixPath": _PP}
    exec(m.group(0), ns2)
    matches = ns2["matches"]
    check("path-lease: allow glob matches", matches("internal/api/handler.go", "internal/api/**"))
    check("path-lease: deny glob matches", matches("migrations/0001_x.sql", "migrations/**"))
    check("path-lease: outside allow does not match", not matches("web/src/App.tsx", "internal/api/**"))
else:  # pragma: no cover
    failures.append("could not extract matches() from check_lease_diff.py")

# --- Review/provenance behavior remains valid (handoff schema still parses) ---
handoff_schema = json.loads(
    (ROOT / "agent-kit/contracts/HANDOFF.schema.json").read_text(encoding="utf-8")
)
jsonschema.Draft202012Validator.check_schema(handoff_schema)
check("handoff schema still valid", True)

# --- Output ---
print(f"PASS: {len(passes)}")
print(f"FAIL: {len(failures)}")
for f in failures:
    print("  FAIL:", f)

sys.exit(1 if failures else 0)

