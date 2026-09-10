#!/usr/bin/env python3
"""Describe or execute a real BookDB stage runner. Missing tests never pass."""
import argparse
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('stage', choices=[f'S{i}' for i in range(9)])
    parser.add_argument('--describe', action='store_true')
    args = parser.parse_args()
    stages = json.loads((ROOT / 'docs/bookdb/stages.json').read_text())
    stage = next(s for s in stages if s['id'] == args.stage)
    if args.describe:
        print(json.dumps(stage, indent=2, ensure_ascii=False))
        return 0
    runner = ROOT / 'scripts/acceptance' / f'{args.stage.lower()}.sh'
    if not runner.is_file() or runner.is_symlink():
        print(f'NOT IMPLEMENTED: {runner.relative_to(ROOT)}. Implement the real stage tests with its application slice.', file=sys.stderr)
        return 2
    print(f'Executing {runner.relative_to(ROOT)}; this does not certify a human gate.', flush=True)
    return subprocess.run(['bash', str(runner)], cwd=ROOT, check=False).returncode

if __name__ == '__main__':
    raise SystemExit(main())
