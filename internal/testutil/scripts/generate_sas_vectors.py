"""Generate independent SAS protocol vectors from pinned Apprise 2.1.0."""
import argparse
import json
from pathlib import Path
import apprise
from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
from apprise.plugins.matrix.e2ee import MatrixSASVerification, _b64enc

parser = argparse.ArgumentParser()
parser.add_argument('--check', action='store_true')
args = parser.parse_args()
assert apprise.__version__ == '2.1.0'
rows = []
for index in range(12):
    own_private = bytes((value + index) % 256 for value in range(32))
    peer_private = bytes((value + 73 + index) % 256 for value in range(32))
    own_user, own_device = '@same:example.com', 'SENDER'
    peer_user, peer_device = own_user, 'PEER'
    start = dict(transaction_id=f'transaction-{index}', from_device=peer_device,
                 method='m.sas.v1', key_agreement_protocols=['curve25519-hkdf-sha256'],
                 hashes=['sha256'], message_authentication_codes=['hkdf-hmac-sha256.v2'],
                 short_authentication_string=['decimal', 'emoji'])
    sas = MatrixSASVerification(own_user, own_device, peer_user, peer_device, start)
    sas._private_key = X25519PrivateKey.from_private_bytes(own_private)
    sas.public_key = _b64enc(sas._private_key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw))
    accept = sas.accept_content()
    peer_public = _b64enc(X25519PrivateKey.from_private_bytes(peer_private).public_key().public_bytes(Encoding.Raw, PublicFormat.Raw))
    sas.receive_key(peer_public)
    code = sas.decimal_sas()
    own_signing, peer_signing = 'test-own-signing-key', 'test-peer-signing-key'
    own_mac = sas.mac_content(own_signing)
    key_id = f'ed25519:{peer_device}'
    peer_keys = {key_id: peer_signing}
    peer_mac = dict(transaction_id=start['transaction_id'], mac={key_id: sas._calculate_mac(peer_signing, key_id, peer_user, peer_device, own_user, own_device)}, keys=sas._calculate_mac(key_id, 'KEY_IDS', peer_user, peer_device, own_user, own_device))
    assert sas.verify_peer_mac(peer_mac, peer_keys)
    rows.append(dict(own_private=own_private.hex(), own_user=own_user, own_device=own_device,
                     peer_user=peer_user, peer_device=peer_device, start=start, accept=accept,
                     peer_public=peer_public, decimal=code, own_signing=own_signing,
                     own_mac=own_mac, peer_keys=peer_keys, peer_mac=peer_mac))
raw = (json.dumps(rows, indent=2) + '\n').encode()
path = Path(__file__).resolve().parents[2] / 'matrixolm/testdata/sas.json'
if args.check:
    assert path.read_bytes() == raw, 'SAS vectors differ from pinned upstream'
else:
    path.write_bytes(raw)
