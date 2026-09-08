"""Exercise actual stamping and review in an isolated synthetic Git repo."""
from pathlib import Path
from tempfile import TemporaryDirectory
import json, os, subprocess
binary=Path('bin/provmark').resolve()
with TemporaryDirectory(prefix='provmark-demo-') as folder:
    root=Path(folder)
    def cmd(*args):
        return subprocess.run(args,cwd=root,text=True,capture_output=True,check=True).stdout
    cmd('git','init','-q')
    cmd('git','config','user.name','Demo Author')
    cmd('git','config','user.email','demo@example.invalid')
    original='first = 1\n'+'# unchanged separator\n'*14+'last = 1\n'
    (root/'example.py').write_text(original)
    cmd('git','add','example.py')
    cmd('git','-c','commit.gpgsign=false','-c','core.hooksPath=/dev/null','commit','-qm','synthetic baseline')
    (root/'example.py').write_text(original.replace('first = 1','first = 2').replace('last = 1','last = 2'))
    (root/'events.jsonl').write_text(json.dumps({'file':'example.py','start_line':16,'end_line':16,'session_id':'synthetic-agent','vendor':'claude-code'})+'\n')
    cmd(str(binary),'stamp','--from','events.jsonl','--dir','.')
    before=json.loads((root/'.provmark.json').read_text())
    agent=next(s for s in before['stamps'] if s['author_kind']=='agent')
    region=agent['region'];target=f"{region['file']}:{region['start']}-{region['end']}"
    print(cmd(str(binary),'review','--dir','.'),end='')
    print(cmd(str(binary),'review','--mark',target,'--dir','.'),end='')
    print(cmd(str(binary),'review','--dir','.'),end='')
    after=json.loads((root/'.provmark.json').read_text())
    print(json.dumps({'human_hunks':sum(s['author_kind']=='human' for s in before['stamps']),'agent_hunks':sum(s['author_kind']=='agent' for s in before['stamps']),'unreviewed_before':sum(s['author_kind']=='agent' and not s['reviewed'] for s in before['stamps']),'unreviewed_after':sum(s['author_kind']=='agent' and not s['reviewed'] for s in after['stamps'])},indent=2))
