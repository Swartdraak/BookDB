#!/usr/bin/env python3
"""Validate one task/lease/handoff/review/ledger/escalation artifact."""
from pathlib import Path
import argparse, json, sys
try:
    import yaml, jsonschema
except Exception as e:
    print("PyYAML and jsonschema required:", e, file=sys.stderr)
    sys.exit(2)

ROOT = Path(__file__).resolve().parents[1]

MAPPING = {
    "task": "TASK_PACKET.schema.json",
    "lease": "AUTHORITY_LEASE.schema.json",
    "handoff": "HANDOFF.schema.json",
    "review": "REVIEW_REPORT.schema.json",
    "ledger": "DELEGATION_LEDGER.schema.json",
    "escalation": "ESCALATION.schema.json",
}

ap = argparse.ArgumentParser()
ap.add_argument("kind", choices=MAPPING)
ap.add_argument("file")
args = ap.parse_args()

data = yaml.safe_load(Path(args.file).read_text(encoding="utf-8"))
schema = json.loads((ROOT/"agent-kit/contracts"/MAPPING[args.kind]).read_text(encoding="utf-8"))

try:
    jsonschema.validate(data, schema)
except Exception as e:
    print(f"FAIL: {e}")
    sys.exit(1)

print("PASS")
