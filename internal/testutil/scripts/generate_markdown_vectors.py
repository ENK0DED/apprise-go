"""Refresh deterministic Markdown expectations from the pinned Apprise source."""
from pathlib import Path
import argparse,gzip,json,random
import apprise
from apprise.common import NotifyFormat
from apprise.conversion.commonmark import commonmark_repair_chunk
from apprise.conversion.dialect import split_dialect_chunk,truncate_dialect_chunk
from apprise.plugins.slack import NotifySlack
from apprise.plugins.google_chat import NotifyGoogleChat
from apprise.plugins.evolution import NotifyEvolution
from apprise.plugins.telegram import NotifyTelegram,TelegramHTMLReducer

root=Path(__file__).resolve().parents[3];data=root/'internal/notify/testdata'
parser=argparse.ArgumentParser();parser.add_argument('--check',action='store_true');args=parser.parse_args()
assert apprise.__version__=='2.1.0','Use the pinned Apprise 2.1.0 reference'
def write(name,value,compressed=False):
 raw=(json.dumps(value,ensure_ascii=False,indent=1)+'\n').encode()
 # Python 3.15 changed the default level from 9 to 6; pin the encoding.
 if compressed:raw=gzip.compress(raw,compresslevel=9,mtime=0)
 path=data/name
 if args.check:
  assert path.read_bytes()==raw,f'{name} differs from upstream; regenerate vectors'
 else:path.write_bytes(raw)
inputs=[x['Input'] for x in json.loads((data/'anchor_markdown.json').read_text())]
rows=[]
for text in inputs:
 rows.append(dict(Input=text,Slack=NotifySlack._commonmark_to_slack(text),Chat=NotifyGoogleChat._commonmark_to_google_chat(text),WhatsApp=NotifyEvolution._commonmark_to_whatsapp(text),Telegram1=NotifyTelegram._commonmark_to_telegram(text),Telegram2=NotifyTelegram._commonmark_to_telegram(text,strict=True)))
write('anchor_markdown.json',rows)
chains=[]
for row in json.loads((data/'cliq_markdown.json').read_text()):
 for width in [1,3,9,25]:
  text=row['input'];pieces=[text[i:i+width] for i in range(0,len(text),width)] or [''];state={};trace=[]
  for i,piece in enumerate(pieces):
   next_piece=pieces[i+1] if i+1<len(pieces) else ''
   fixed,state=commonmark_repair_chunk(piece,state,next_chunk=next_piece or None)
   trace.append(dict(input=piece,next=next_piece,output=fixed,state=state.copy()))
  chains.append(trace)
write('commonmark_repair.json.gz',chains,True)
random.Random(31).shuffle(inputs);rows=[]
for text in inputs[:100]:
 for limit in [3,11,27]:
  for name,fn in [('slack',NotifySlack._commonmark_to_slack),('chat',NotifyGoogleChat._commonmark_to_google_chat)]:
   rows.append(dict(Input=text,Limit=limit,Dialect=name,Split=split_dialect_chunk(text,limit,fn,NotifyFormat.MARKDOWN),Truncate=truncate_dialect_chunk(text,limit,fn,NotifyFormat.MARKDOWN)))
from html import escape
for text in ['a &CounterClockwiseContourIntegral; z', 'a <a href="space > end">label</a> z', '<b>long text</b> and &amp; tail', '<unclosed attr="x', '<ul><li>one</li><li>two</li></ul>', 'emoji 🐧 & <b> > " quote']:
 for limit in [1,3,11,27]:
  for name,fmt,fn in [('telegram-html',NotifyFormat.HTML,lambda value: TelegramHTMLReducer().reduce(value)),('slack-text',NotifyFormat.TEXT,lambda value: escape(value,quote=False))]:
   rows.append(dict(Input=text,Limit=limit,Dialect=name,Split=split_dialect_chunk(text,limit,fn,fmt),Truncate=truncate_dialect_chunk(text,limit,fn,fmt)))
write('dialect_overflow.json',rows)
rows=json.loads((data/'telegram_html_reduce.json').read_text())
write('telegram_html_reduce.json',[dict(Input=row['Input'],Output=TelegramHTMLReducer().reduce(row['Input'])) for row in rows])
from apprise.utils.format import smart_split
split_inputs=inputs+['', '     a b c', '\r\n \t\va!\fnext', 'a &CounterClockwiseContourIntegral; z', 'a <tag attr="space > quote">body</tag> tail', '<tag attr="a > b">hello</tag>', 'a [label](https://example.com) z', 'a ![image](https://example.com) z', 'a <https://example.com|label> z', 'a\\*b\\\\*c']
rows=[]
for text in split_inputs:
 for limit in [-1,1,3,11,27,60]:
  for fmt in [NotifyFormat.TEXT,NotifyFormat.HTML,NotifyFormat.MARKDOWN]:
   rows.append(dict(Input=text,Format=fmt.value,Limit=limit,Output=smart_split(text,limit,fmt)))
write('smart_split.json.gz',rows,True)
