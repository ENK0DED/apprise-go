"""Decrypt PGP/MIME and verify signatures before comparing recovered content."""
import json,sys,re
from pathlib import Path
from email import policy
from email.parser import BytesParser
from apprise.utils import pgp
import pgpy

payload=json.loads(sys.argv[1]);raw=payload['message'].encode();root=Path(__file__).resolve().parents[1]/'fixtures'
recipient,_=pgpy.PGPKey.from_file(str(root/'pgp_recipient_private.asc'))
sender,_=pgpy.PGPKey.from_file(str(root/'pgp_sender_public.asc'))
encrypted=False;signed=False
for _ in range(3):
 message=BytesParser(policy=policy.SMTP).parsebytes(raw)
 if message.get_content_type()=='multipart/encrypted':
  encrypted=True;parts=list(message.iter_parts());assert len(parts)==2 and parts[0].get_content_type()=='application/pgp-encrypted'
  cipher=pgpy.PGPMessage.from_blob(parts[1].get_payload(decode=True));plain=recipient.decrypt(cipher).message
  raw=plain.encode() if isinstance(plain,str) else bytes(plain);continue
 if message.get_content_type()=='multipart/signed':
  signed=True;boundary=message.get_boundary().encode();parts=raw.split(b'--'+boundary)
  first=parts[1];assert first.startswith(b'\r\n') or first.startswith(b'\n');first=first[2:] if first.startswith(b'\r\n') else first[1:]
  first=first[:-2] if first.endswith(b'\r\n') else first[:-1]
  signature=pgpy.PGPSignature.from_blob(list(message.iter_parts())[1].get_payload(decode=True))
  canonical=first.replace(b'\r\n',b'\n').replace(b'\n',b'\r\n')
  assert sender.verify(canonical,signature),'PGP signature does not match MIME part'
  raw=first;continue
 break
message=BytesParser(policy=policy.SMTP).parsebytes(raw)
parts=[]
for part in message.walk():
 if part.is_multipart():continue
 data=part.get_payload(decode=True) or b''
 parts.append({'type':part.get_content_type(),'name':part.get_filename(),'body':data.decode(errors='replace').replace('\r\n','\n')})
print(json.dumps({'encrypted':encrypted,'signed':signed,'parts':parts},sort_keys=True))
