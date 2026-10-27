package matrixolm

import (
	"crypto/ecdh"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSASUpstreamCryptographicVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/sas.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		OwnPrivate    string `json:"own_private"`
		OwnUser       string `json:"own_user"`
		OwnDevice     string `json:"own_device"`
		PeerUser      string `json:"peer_user"`
		PeerDevice    string `json:"peer_device"`
		Start, Accept map[string]any
		PeerPublic    string `json:"peer_public"`
		Decimal       [3]int
		OwnSigning    string            `json:"own_signing"`
		OwnMAC        map[string]any    `json:"own_mac"`
		PeerKeys      map[string]string `json:"peer_keys"`
		PeerMAC       map[string]any    `json:"peer_mac"`
	}
	if err = json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		fresh := func() *SASVerification {
			s, err := NewSASVerification(v.OwnUser, v.OwnDevice, v.PeerUser, v.PeerDevice, v.Start)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := hex.DecodeString(v.OwnPrivate)
			if err != nil {
				t.Fatal(err)
			}
			s.private, err = ecdh.X25519().NewPrivateKey(raw)
			if err != nil {
				t.Fatal(err)
			}
			s.PublicKey = EncodeBase64(s.private.PublicKey().Bytes())
			return s
		}
		s := fresh()
		accept, err := s.AcceptContent()
		if err != nil {
			t.Fatal(err)
		}
		got, _ := CanonicalJSON(accept)
		want, _ := CanonicalJSON(v.Accept)
		if string(got) != string(want) {
			t.Fatalf("commitment Go%s Python%s", got, want)
		}
		if _, err = s.AcceptContent(); err == nil {
			t.Fatal("accepted repeated start")
		}
		if err = s.ReceiveKey(v.PeerPublic); err != nil {
			t.Fatal(err)
		}
		if code, err := s.DecimalSAS(); err != nil || !reflect.DeepEqual(code, v.Decimal) {
			t.Fatalf("SAS code%v want%v err%v", code, v.Decimal, err)
		}
		mac, err := s.MACContent(v.OwnSigning)
		if err != nil {
			t.Fatal(err)
		}
		got, _ = CanonicalJSON(mac)
		want, _ = CanonicalJSON(v.OwnMAC)
		if string(got) != string(want) {
			t.Fatalf("MAC Go%s Python%s", got, want)
		}
		if err = s.VerifyPeerMAC(v.PeerMAC, v.PeerKeys); err != nil || !s.Verified() {
			t.Fatalf("upstream MAC rejected: %v", err)
		}
		for _, mutation := range []string{"transaction", "keys", "device-mac", "unknown-key"} {
			s = fresh()
			_, _ = s.AcceptContent()
			if err = s.ReceiveKey(v.PeerPublic); err != nil {
				t.Fatal(err)
			}
			_, _ = s.MACContent(v.OwnSigning)
			raw, _ := json.Marshal(v.PeerMAC)
			var changed map[string]any
			_ = json.Unmarshal(raw, &changed)
			switch mutation {
			case "transaction":
				changed["transaction_id"] = "other"
			case "keys":
				changed["keys"] = "tampered"
			case "device-mac":
				changed["mac"].(map[string]any)["ed25519:"+v.PeerDevice] = "tampered"
			case "unknown-key":
				v.PeerKeys = map[string]string{}
			}
			if err = s.VerifyPeerMAC(changed, v.PeerKeys); err == nil || s.Verified() {
				t.Fatalf("accepted %s", mutation)
			}
		}
	}
}
