package notify

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// The fixture comes from upstream 2.1.0's actual password derivation and
// PyNaCl encryption, including its integration-token key wrapping.
func TestSimplePushCryptoUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/simplepush_crypto.json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]string
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	decode := func(name string) []byte {
		t.Helper()
		value, err := hex.DecodeString(v[name])
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	key, fingerprint := deriveSimplePushKey(v["password"], v["salt"])
	if !bytes.Equal(key, decode("key")) || fingerprint != v["fingerprint"] {
		t.Fatal("password derivation differs from upstream")
	}
	t.Setenv("APPRISE_SIMPLEPUSH_TEST_NONCE", v["nonce"])
	sealed, err := sealSimplePush([]byte(v["body"]), key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sealed, decode("sealed")) {
		t.Fatal("XChaCha20-Poly1305 envelope differs from upstream")
	}
	opened, err := openSimplePushOrganizationKey(decode("seed"), decode("admin_public"), decode("wrapped"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, decode("organization_key")) {
		t.Fatal("organization key differs from upstream")
	}
	wrapped := decode("wrapped")
	wrapped[len(wrapped)-1] ^= 1
	if _, err := openSimplePushOrganizationKey(decode("seed"), decode("admin_public"), wrapped); err == nil {
		t.Fatal("accepted a tampered organization key")
	}
	seed := decode("seed")
	seed[0] ^= 1
	if _, err := openSimplePushOrganizationKey(seed, decode("admin_public"), decode("wrapped")); err == nil {
		t.Fatal("accepted the wrong integration seed")
	}
}
