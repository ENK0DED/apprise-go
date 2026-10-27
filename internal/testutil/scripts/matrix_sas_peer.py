"""Independent same-user SAS peer for the real HTTP verification test."""
import json
import secrets
import sys
from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey, X25519PublicKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
from apprise.plugins.matrix.e2ee import MatrixOlmAccount, MatrixSASVerification, _b64enc, _b64dec

input_data = json.loads(sys.argv[1])
user, own_device, peer_device = '@same:example.com', 'SENDER', 'PEER'
peer_private = X25519PrivateKey.from_private_bytes(bytes(range(73, 105)))
peer_public = _b64enc(peer_private.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw))
peer_account = MatrixOlmAccount(ik_priv_b64=_b64enc(bytes(range(32))), sk_priv_b64=_b64enc(bytes(range(32, 64))))
if input_data['operation'] == 'keys':
    print(json.dumps(dict(device=peer_account.device_keys_payload(user, peer_device), public_key=peer_public)))
else:
    start, accept = input_data['start'], input_data['accept']
    own_public, own_mac = input_data['key']['key'], input_data['mac']
    own_keys = input_data['device_keys']
    assert own_keys['user_id'] == user and own_keys['device_id'] == own_device
    sas = MatrixSASVerification(user, own_device, user, peer_device, start)
    sas.public_key = own_public
    expected_accept = sas.accept_content()
    assert secrets.compare_digest(accept['commitment'], expected_accept['commitment'])
    assert accept == expected_accept
    sas._shared_secret = peer_private.exchange(X25519PublicKey.from_public_bytes(_b64dec(own_public)))
    sas._peer_public_key = peer_public
    sas.state = 'key_received'
    assert sas.mac_content(own_keys['keys']['ed25519:' + own_device]) == own_mac
    key_id = 'ed25519:' + peer_device
    peer_mac = dict(transaction_id=start['transaction_id'], mac={key_id: sas._calculate_mac(peer_account.signing_key, key_id, user, peer_device, user, own_device)}, keys=sas._calculate_mac(key_id, 'KEY_IDS', user, peer_device, user, own_device))
    print(json.dumps(peer_mac))
