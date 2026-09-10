#!/usr/bin/env python3
"""Publish owned Wiki navigation pages from clean committed main; preview by default."""
import argparse
from pathlib import Path
import re
import subprocess
import sys
import tempfile

ROOT=Path(__file__).resolve().parents[2]
MARKER='<!-- generated-by: bookdb-project-package -->'

def run(args,cwd=None):
    result=subprocess.run(args,cwd=cwd,text=True,capture_output=True,check=False,timeout=120)
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or 'Command failed')
    return result.stdout.strip()

def pages(repo):
    base=f'https://github.com/{repo}/blob/main/'
    groups={
        'BookDB-Home.md': [('Project handbook','BOOKDB_PROJECT.md'),('Product requirements','docs/bookdb/01-product-requirements.md'),('Delivery plan','docs/bookdb/07-delivery-plan.md')],
        'BookDB-Development.md': [('Contributing','CONTRIBUTING.md'),('Testing','docs/bookdb/08-testing.md'),('JetBrains setup','docs/bookdb/11-jetbrains-tooling.md'),('API contract','docs/bookdb/05-api-security.md'),('Bibliophilarr integration','docs/bookdb/18-bibliophilarr-integration.md')],
        'BookDB-Operations.md': [('Operations and release','docs/bookdb/12-operations-release.md'),('GitHub operations','docs/bookdb/09-github-operations.md'),('Security','SECURITY.md'),('Research','docs/bookdb/17-references.md')]
    }
    result={}
    for filename,entries in groups.items():
        result[filename]=MARKER+'\n# '+filename[:-3].replace('-',' ')+'\n\nCanonical specifications live in the repository. Main may describe planned behavior; use release-tag documentation for released versions.\n\n'+'\n'.join(f'- [{label}]({base}{path})' for label,path in entries)+'\n'
    return result

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--repo',default='Swartdraak/BookDB')
    p.add_argument('--apply',action='store_true')
    args=p.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+',args.repo):
        p.error('Use owner/name.')
    content=pages(args.repo)
    if not args.apply:
        for name in content:
            print(f'ENSURE owned Wiki page: {name}')
        print('Preview only. Existing unrelated Wiki pages will be preserved.')
        return 0
    try:
        if run(['git','status','--porcelain'],ROOT):
            raise RuntimeError('Wiki publication requires a clean committed source checkout.')
        head=run(['git','rev-parse','HEAD'],ROOT)
        remote=run(['git','ls-remote',f'https://github.com/{args.repo}.git','refs/heads/main'],ROOT).split()[0]
        if head != remote:
            raise RuntimeError('Wiki publication requires the exact current remote main commit.')
        with tempfile.TemporaryDirectory(prefix='bookdb-wiki-') as temp:
            wiki=Path(temp)/'wiki'
            try:
                run(['git','clone','--depth','1',f'https://github.com/{args.repo}.wiki.git',str(wiki)])
            except RuntimeError as exc:
                raise RuntimeError('Wiki unavailable: verify credentials and create its first page through GitHub if needed. '+str(exc)) from exc
            # Validate all destinations before changing any.
            for name in content:
                dest=wiki/name
                if dest.is_symlink() or (dest.exists() and MARKER not in dest.read_text()):
                    raise RuntimeError(f'Preserving unowned/conflicting Wiki page {name}; reconcile explicitly.')
            for name,text in content.items():
                (wiki/name).write_text(text)
            run(['git','add','--',*content.keys()],wiki)
            if not run(['git','diff','--cached','--name-only'],wiki):
                print('Wiki navigation already current.')
                return 0
            run(['git','-c','user.name=BookDB documentation automation','-c','user.email=bookdb-docs@users.noreply.github.com',
                 'commit','-m',f'docs: sync BookDB navigation from {head[:12]}'],wiki)
            run(['git','push','origin','HEAD'],wiki)
        print('Owned Wiki navigation pages synchronized; unrelated pages preserved.')
        return 0
    except (RuntimeError,FileNotFoundError,subprocess.TimeoutExpired,IndexError) as exc:
        print(f'Wiki sync incomplete: {exc}',file=sys.stderr)
        return 1

if __name__=='__main__':
    raise SystemExit(main())
