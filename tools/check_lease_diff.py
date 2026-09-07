#!/usr/bin/env python3
"""
Validate that Git changes are contained within an AuthorityLease.

Usage:
  python tools/check_lease_diff.py \
      --lease .agent-state/leases/lease-M0-API-001-001.yaml \
      --base <base-commit>

By default compares BASE...HEAD.
"""
from pathlib import Path
from pathlib import PurePosixPath
import argparse, subprocess, sys
try:
    import yaml
except Exception as e:
    print("PyYAML required:", e, file=sys.stderr)
    sys.exit(2)

def matches(path: str, pattern: str) -> bool:
    # Support parameterized paths only after lease has concretized them.
    if "<" in pattern or ">" in pattern:
        return False
    p = PurePosixPath(path)
    # pathlib semantics plus simple root-file glob compatibility
    return p.match(pattern) or (
        pattern.endswith("/**") and (path == pattern[:-3].rstrip("/") or path.startswith(pattern[:-3]))
    )

ap = argparse.ArgumentParser()
ap.add_argument("--lease", required=True)
ap.add_argument("--base", default=None)
ap.add_argument("--head", default="HEAD")
args = ap.parse_args()

lease = yaml.safe_load(Path(args.lease).read_text(encoding="utf-8"))
if lease.get("status") != "ACTIVE":
    print(f"ERROR: lease status is {lease.get('status')}, expected ACTIVE")
    sys.exit(1)

base = args.base or lease.get("base_commit")
if not base:
    print("ERROR: no base commit")
    sys.exit(2)

base_file = subprocess.run(
    ["git", "show", f"{base}:{args.lease}"],
    capture_output=True,
    text=True,
)
if base_file.returncode == 0:
    base_lease = yaml.safe_load(base_file.stdout) or {}
    for field in ("status", "write_allow", "write_deny", "branch", "base_commit"):
        if base_lease.get(field) != lease.get(field):
            print(
                f"ERROR: lease {args.lease} changed {field} from "
                f"{base_lease.get(field)!r} to {lease.get(field)!r}; "
                "forbidden without explicit governance approval"
            )
            sys.exit(1)

proc = subprocess.run(
    ["git","diff","--name-only",f"{base}...{args.head}"],
    capture_output=True, text=True
)
if proc.returncode:
    print(proc.stderr)
    sys.exit(proc.returncode)

changed = [x.strip().replace("\\","/") for x in proc.stdout.splitlines() if x.strip()]
allow = lease.get("write_allow", [])
deny = lease.get("write_deny", [])

violations = []
for path in changed:
    denied = any(matches(path, pat) for pat in deny)
    allowed = any(matches(path, pat) for pat in allow)
    if denied or not allowed:
        violations.append((path, "explicitly denied" if denied else "outside write_allow"))

print(f"Lease: {lease.get('lease_id')}")
print(f"Agent: {lease.get('agent_id')}")
print(f"Changed files: {len(changed)}")

if violations:
    print("LEASE VIOLATIONS:")
    for path, reason in violations:
        print(f"  - {path}: {reason}")
    sys.exit(1)

print("PASS: all changed files are within lease.")
