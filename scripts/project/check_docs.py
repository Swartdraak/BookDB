#!/usr/bin/env python3
"""Check active Markdown file links. No task-state or lease validation."""
import argparse
import json
from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[2]
LINK = re.compile(r'(?<!!)\[[^\]\n]+\]\(([^)\n]+)\)')

def check(root, preserved=()):
    errors = []
    paths = [root / n for n in ['README.md', 'BOOKDB_PROJECT.md', 'AGENTS.md', 'CONTRIBUTING.md', 'SECURITY.md']]
    paths += sorted((root / 'docs/bookdb').glob('*.md'))
    paths += [root / 'adrs/0011-delivery-and-governance-reset.md']
    for path in paths:
        if not path.is_file():
            errors.append(f'Missing active document: {path.relative_to(root)}')
            continue
        text = re.sub(r'```.*?```', '', path.read_text(), flags=re.S)
        for match in LINK.finditer(text):
            target = match.group(1).strip().split(' "', 1)[0].strip('<>')
            if not target or target.startswith('#') or urlsplit(target).scheme:
                continue
            raw = unquote(target.split('#', 1)[0])
            resolved = (path.parent / raw).resolve()
            if not resolved.is_relative_to(root.resolve()):
                errors.append(f'{path.relative_to(root)}: link escapes repo: {target}')
            elif not resolved.exists() and str(resolved.relative_to(root.resolve())) not in preserved:
                errors.append(f'{path.relative_to(root)}: missing target: {target}')
    stages_path = root / 'docs/bookdb/stages.json'
    if stages_path.exists():
        try:
            stages = json.loads(stages_path.read_text())
            ids = [s['id'] for s in stages]
            if ids != [f'S{i}' for i in range(9)]:
                errors.append('Stage IDs must remain the declared S0-S8 sequence or be deliberately updated with this check.')
            for s in stages:
                if not s['tests'] or not s['human'] or not s['done']:
                    errors.append(f'{s["id"]}: incomplete acceptance contract')
        except (KeyError, ValueError, TypeError) as exc:
            errors.append(f'Invalid stage contract: {exc}')
    return errors

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--package', action='store_true', help='Validate overlay links to explicitly preserved baseline files.')
    args = p.parse_args()
    preserved = set()
    if args.package:
        manifest = ROOT.parent / 'migration-manifest.json'
        if not manifest.is_file():
            p.error('Package manifest is required for --package.')
        preserved = set(json.loads(manifest.read_text())['preserved_paths'])
    errors = check(ROOT, preserved)
    for e in errors:
        print(e, file=sys.stderr)
    print(f'Active documentation: {len(errors)} error(s). Application behavior is not tested here.')
    return int(bool(errors))

if __name__ == '__main__':
    raise SystemExit(main())
