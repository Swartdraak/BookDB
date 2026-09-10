#!/usr/bin/env python3
"""Preview/create missing BookDB GitHub planning objects. No merges, releases or gate approval."""
import argparse
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
REPO_RE = re.compile(r'^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$')

def gh(args, payload=None):
    command = ['gh', *args]
    if payload is not None:
        command += ['--input', '-']
    result = subprocess.run(command, input=json.dumps(payload) if payload is not None else None,
                            text=True, capture_output=True, check=False, timeout=120)
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or 'GitHub command failed')
    return json.loads(result.stdout) if result.stdout.strip() else None

def pages(path):
    result = gh(['api', path, '--paginate'])
    if isinstance(result, list):
        return result
    return [entry for page in result for entry in page]

def issue_body(stage, code, scope):
    marker = f'<!-- bookdb-package:{stage["id"]}-{code} -->'
    tests = '\n'.join(f'- [ ] {t}' for t in stage['tests'])
    return f'''{marker}
## Outcome
{scope}

## Stage and requirements
{stage['id']} — {stage['name']}
Requirements: {', '.join(stage['requirements'])}
Dependencies: {', '.join(stage['requires']) or 'baseline adoption'}

## Dataset/environment
{stage['dataset']}

## Stage acceptance contract
Refine this issue to its owned subset before implementation; all stage criteria must be covered across the milestone.
{tests}

## Human-runnable procedure
{stage['human']}

## Completion boundary
{stage['done']}

## Evidence
NOT RUN. Record actual commands/CI/results here or in the linked PR.

## Human acceptance
{'Required for this gate issue; only the human can provide the result.' if code == 'ACCEPT' else 'Do not manufacture human signoff. Follow the stage gate policy.'}
'''

def setup(repo, apply=False, project=False):
    if not REPO_RE.fullmatch(repo):
        raise ValueError('Repository must be owner/name.')
    owner, name = repo.split('/')
    metadata = gh(['api', f'repos/{repo}'])
    if metadata['full_name'].lower() != repo.lower():
        raise ValueError('Repository identity mismatch.')
    stages = json.loads((ROOT / 'docs/bookdb/stages.json').read_text())
    labels = {x['name']: x for x in pages(f'repos/{repo}/labels?per_page=100')}
    milestones = pages(f'repos/{repo}/milestones?state=all&per_page=100')
    issues = [x for x in pages(f'repos/{repo}/issues?state=all&per_page=100') if 'pull_request' not in x]
    definitions = {}
    for prefix, values, color in [
        ('type', ['feature','bug','maintenance','documentation','data-quality','source','acceptance'], '0366d6'),
        ('area', ['backend','database','ingestion','webui','api','security','operations','integration'], '5319e7'),
        ('priority', ['p0','p1','p2','p3'], 'd93f0b'),
        ('status', ['backlog','ready','in-progress','review','human-testing','blocked'], '0e8a16'),
        ('gate', ['human'], 'fbca04')]:
        for value in values:
            definitions[f'{prefix}:{value}'] = {'name':f'{prefix}:{value}', 'color':color, 'description':f'BookDB {prefix}: {value}'}
    for key, payload in definitions.items():
        if key not in labels:
            print(f'CREATE label {key}')
            if apply:
                gh(['api', '-X', 'POST', f'repos/{repo}/labels'], payload)
    settings = {'has_issues':True, 'has_projects':True, 'has_wiki':True, 'has_discussions':True,
                'allow_squash_merge':True, 'delete_branch_on_merge':True}
    needed = {k:v for k,v in settings.items() if metadata.get(k) != v}
    if needed:
        print(f'UPDATE repository settings: {", ".join(needed)}')
        if apply:
            gh(['api', '-X', 'PATCH', f'repos/{repo}'], needed)
    issue_urls = []
    for stage in stages:
        title = f'{stage["id"]} — {stage["name"]}'
        matches = [m for m in milestones if m['title'] == title]
        if len(matches) > 1:
            raise ValueError(f'Ambiguous milestone: {title}')
        milestone = matches[0] if matches else None
        if milestone is None:
            print(f'CREATE milestone {title}')
            if apply:
                milestone = gh(['api', '-X', 'POST', f'repos/{repo}/milestones'],
                               {'title':title, 'description':stage['done']})
        for code, issue_title, scope in stage['issues']:
            marker = f'<!-- bookdb-package:{stage["id"]}-{code} -->'
            found = [i for i in issues if marker in (i.get('body') or '')]
            if len(found) > 1:
                raise ValueError(f'Duplicate issue marker: {marker}')
            if found:
                issue_urls.append(found[0]['html_url'])
                continue
            print(f'CREATE issue {stage["id"]}-{code}: {issue_title}')
            if apply:
                labs = ['type:acceptance' if code == 'ACCEPT' else 'type:feature',
                        'status:ready' if stage['id'] == 'S0' else 'status:backlog', 'priority:p1']
                if code == 'ACCEPT':
                    labs.append('gate:human')
                item = gh(['api', '-X', 'POST', f'repos/{repo}/issues'],
                          {'title':f'[{stage["id"]}-{code}] {issue_title}', 'body':issue_body(stage,code,scope),
                           'labels':labs, 'milestone':milestone['number']})
                issue_urls.append(item['html_url'])
    if project:
        projects = gh(['project','list','--owner',owner,'--limit','1000','--format','json'])['projects']
        if len(projects) >= 1000:
            raise ValueError('Project enumeration reached its safety limit; resolve the intended project explicitly.')
        found = [p for p in projects if p['title'] == 'BookDB delivery']
        if len(found) > 1:
            raise ValueError('More than one BookDB delivery project; resolve identity before mutation.')
        proj = found[0] if found else None
        if not proj:
            print('CREATE Project BookDB delivery')
            if apply:
                proj = gh(['project','create','--owner',owner,'--title','BookDB delivery','--format','json'])
        if proj:
            number = str(proj['number'])
            print(f'LINK Project {number} and ensure planned items/fields')
            fields = gh(['project','field-list',number,'--owner',owner,'--limit','100','--format','json'])['fields']
            if apply:
                gh(['project','link',number,'--owner',owner,'--repo',repo])
            for field, opts in [('Priority',['P0','P1','P2','P3']), ('Stage',[s['id'] for s in stages]), ('Gate',['Automated','Human'])]:
                existing = [f for f in fields if f['name'] == field]
                if not existing:
                    print(f'CREATE Project field {field}')
                    if apply:
                        gh(['project','field-create',number,'--owner',owner,'--name',field,
                            '--data-type','SINGLE_SELECT','--single-select-options',','.join(opts),'--format','json'])
            # item-add is idempotent for an issue already belonging to the Project.
            for url in issue_urls:
                print(f'ENSURE Project item {url}')
                if apply:
                    gh(['project','item-add',number,'--owner',owner,'--url',url,'--format','json'])
    print('Applied missing planning objects.' if apply else 'Preview only; no mutations performed.')
    print('Set views/field values and verify protections/check names per the GitHub runbook. No human gate was approved.')

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--repo', default='Swartdraak/BookDB')
    p.add_argument('--apply', action='store_true')
    p.add_argument('--project', action='store_true')
    args=p.parse_args()
    try:
        setup(args.repo,args.apply,args.project)
        return 0
    except (RuntimeError,ValueError,KeyError,subprocess.TimeoutExpired,FileNotFoundError) as exc:
        print(f'INCOMPLETE: {exc}. Completed operations are retained; resolve the blocker and rerun safely.',file=sys.stderr)
        return 1

if __name__ == '__main__':
    raise SystemExit(main())
