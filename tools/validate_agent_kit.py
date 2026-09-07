#!/usr/bin/env python3
from pathlib import Path
import json, sys, fnmatch
try:
    import yaml
except Exception as e:
    print("ERROR: PyYAML is required:", e)
    sys.exit(2)
try:
    import jsonschema
except Exception as e:
    print("ERROR: jsonschema is required:", e)
    sys.exit(2)

ROOT = Path(__file__).resolve().parents[1]
errors = []
warnings = []

def load_yaml(p):
    try:
        return yaml.safe_load(p.read_text(encoding="utf-8"))
    except Exception as e:
        errors.append(f"{p}: YAML error: {e}")
        return None

def load_json(p):
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except Exception as e:
        errors.append(f"{p}: JSON error: {e}")
        return None

# Validate schemas parse.
schemas = {}
for p in (ROOT/"agent-kit/contracts").glob("*.schema.json"):
    data = load_json(p)
    if data:
        try:
            jsonschema.Draft202012Validator.check_schema(data)
            schemas[p.name] = data
        except Exception as e:
            errors.append(f"{p}: invalid JSON Schema: {e}")

# Validate examples against schemas.
pairs = [
    ("examples/task-packet.yaml", "TASK_PACKET.schema.json"),
    ("examples/authority-lease.yaml", "AUTHORITY_LEASE.schema.json"),
    ("examples/handoff.yaml", "HANDOFF.schema.json"),
    ("examples/review-report.yaml", "REVIEW_REPORT.schema.json"),
    ("examples/delegation-ledger.yaml", "DELEGATION_LEDGER.schema.json"),
]
for rel, schema_name in pairs:
    p = ROOT/rel
    data = load_yaml(p)
    schema = schemas.get(schema_name)
    if data is not None and schema:
        try:
            jsonschema.validate(data, schema)
        except Exception as e:
            errors.append(f"{p}: schema validation failed: {e}")

# Load agent IDs.
agent_files = list((ROOT/"agent-kit").glob("**/*.agent.md"))
agent_ids = set()
agent_class = {}
for p in agent_files:
    txt = p.read_text(encoding="utf-8")
    m = __import__("re").search(r"^agent_id:\s*([a-z0-9-]+)\s*$", txt, __import__("re").M)
    c = __import__("re").search(r"^class:\s*([A-Z_]+)\s*$", txt, __import__("re").M)
    if not m:
        errors.append(f"{p}: missing agent_id")
        continue
    aid = m.group(1)
    if aid in agent_ids:
        errors.append(f"duplicate agent_id: {aid}")
    agent_ids.add(aid)
    if c:
        agent_class[aid] = c.group(1)

# Ensure orchestrator is control plane.
if agent_class.get("orchestrator") != "CONTROL_PLANE":
    errors.append("orchestrator must be CONTROL_PLANE")

# Routing refs.
routing = load_yaml(ROOT/"agent-kit/routing/ROUTING_TABLE.yaml")
if routing:
    for r in routing.get("routes", []):
        for key in ("lead",):
            a = r.get(key)
            if a and a not in agent_ids:
                errors.append(f"routing references unknown lead agent: {a}")
        for key in ("planning","assurance"):
            for a in r.get(key, []):
                # Some entries use conditional prose; only validate clean IDs.
                if " " not in a and a not in agent_ids:
                    errors.append(f"routing references unknown {key} agent: {a}")

# Ownership refs and exact-pattern collision detection.
ownership = load_yaml(ROOT/"agent-kit/routing/PATH_OWNERSHIP.yaml")
if ownership:
    seen = {}
    for o in ownership.get("owners", []):
        a = o["agent"]
        if a not in agent_ids:
            errors.append(f"path ownership references unknown agent: {a}")
        for pat in o.get("paths", []):
            if pat in seen and seen[pat] != a:
                errors.append(f"exact path ownership collision: {pat}: {seen[pat]} vs {a}")
            seen[pat] = a

# Workflow parse and agent references.
for p in (ROOT/"agent-kit/workflows").glob("*.yaml"):
    data = load_yaml(p)
    if not data:
        continue
    for t in data.get("tasks", []) or []:
        a = t.get("route")
        if a and a not in agent_ids:
            errors.append(f"{p}: unknown task route agent {a}")
        for a in t.get("reviewers", []) or []:
            if a not in agent_ids:
                errors.append(f"{p}: unknown reviewer {a}")

# Critical anti-drift phrases.
orch = (ROOT/"agent-kit/control-plane/orchestrator.agent.md").read_text(encoding="utf-8")
required_phrases = [
    "does not implement BookDB",
    "MUST NOT modify",
    "If `CAN_INVOKE_SUBAGENTS=false`",
    "never fix it yourself",
]
for phrase in required_phrases:
    if phrase not in orch:
        errors.append(f"orchestrator anti-drift rule missing phrase: {phrase}")

# Output.
print(f"Agents: {len(agent_ids)}")
print(f"Schemas: {len(schemas)}")
print(f"Workflows: {len(list((ROOT/'agent-kit/workflows').glob('*.yaml')))}")
print(f"Errors: {len(errors)}")
print(f"Warnings: {len(warnings)}")
for w in warnings:
    print("WARN:", w)
for e in errors:
    print("ERROR:", e)

sys.exit(1 if errors else 0)
