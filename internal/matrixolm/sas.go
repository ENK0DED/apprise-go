package matrixolm

import (
	"bytes"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const SASMethod = "m.sas.v1"

// SASVerification responds to a same-user verification using a fresh X25519
// key, a commitment to the exact proposal and device-bound SHA256 MACs.
type SASVerification struct {
	ownUser, ownDevice, peerUser, peerDevice, transaction string
	start                                                 map[string]any
	private                                               *ecdh.PrivateKey
	PublicKey                                             string
	shared                                                []byte
	peerPublic, state                                     string
}

func sasField(value string) bool { return value != "" && utf8.RuneCountInString(value) <= 512 }
func sasContains(value any, want string) bool {
	switch values := value.(type) {
	case []any:
		for _, v := range values {
			if v == want {
				return true
			}
		}
	case []string:
		for _, v := range values {
			if v == want {
				return true
			}
		}
	}
	return false
}
func NewSASVerification(ownUser, ownDevice, peerUser, peerDevice string, start map[string]any) (*SASVerification, error) {
	transaction, _ := start["transaction_id"].(string)
	if !sasField(transaction) || !sasField(peerDevice) || start["from_device"] != peerDevice || start["method"] != SASMethod || !sasContains(start["key_agreement_protocols"], "curve25519-hkdf-sha256") || !sasContains(start["hashes"], "sha256") || !sasContains(start["message_authentication_codes"], "hkdf-hmac-sha256.v2") || !sasContains(start["short_authentication_string"], "decimal") {
		return nil, fmt.Errorf("unsupported SAS proposal")
	}
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	// Copy through canonical JSON so later caller mutations cannot alter trust.
	raw, err := CanonicalJSON(start)
	if err != nil {
		return nil, err
	}
	proposal, err := decodeSASProposal(raw)
	if err != nil {
		return nil, err
	}
	return &SASVerification{ownUser: ownUser, ownDevice: ownDevice, peerUser: peerUser, peerDevice: peerDevice, transaction: transaction, start: proposal, private: private, PublicKey: EncodeBase64(private.PublicKey().Bytes()), state: "started"}, nil
}
func (s *SASVerification) AcceptContent() (map[string]any, error) {
	if s.state != "started" {
		return nil, fmt.Errorf("SAS proposal already accepted")
	}
	raw, err := CanonicalJSON(s.start)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(append([]byte(s.PublicKey), raw...))
	s.state = "accepted"
	return map[string]any{"transaction_id": s.transaction, "key_agreement_protocol": "curve25519-hkdf-sha256", "hash": "sha256", "message_authentication_code": "hkdf-hmac-sha256.v2", "short_authentication_string": []string{"decimal"}, "commitment": EncodeBase64(digest[:])}, nil
}
func (s *SASVerification) ReceiveKey(value string) error {
	if s.state != "accepted" || !sasField(value) {
		return fmt.Errorf("unexpected SAS public key")
	}
	raw, err := DecodeBase64(value)
	if err != nil {
		return err
	}
	peer, err := ecdh.X25519().NewPublicKey(raw)
	if err != nil {
		return err
	}
	shared, err := s.private.ECDH(peer)
	if err != nil {
		return err
	}
	s.shared = shared
	s.peerPublic = value
	s.state = "key_received"
	return nil
}
func (s *SASVerification) KeyContent() (map[string]any, error) {
	if s.state != "key_received" {
		return nil, fmt.Errorf("SAS peer key missing")
	}
	return map[string]any{"transaction_id": s.transaction, "key": s.PublicKey}, nil
}
func (s *SASVerification) DecimalSAS() ([3]int, error) {
	if s.shared == nil || s.peerPublic == "" {
		return [3]int{}, fmt.Errorf("SAS shared secret missing")
	}
	info := "MATRIX_KEY_VERIFICATION_SAS|" + strings.Join([]string{s.peerUser, s.peerDevice, s.peerPublic, s.ownUser, s.ownDevice, s.PublicKey, s.transaction}, "|")
	raw, err := hkdf.Key(sha256.New, s.shared, nil, info, 5)
	if err != nil {
		return [3]int{}, err
	}
	value := uint64(0)
	for _, b := range raw {
		value = value<<8 | uint64(b)
	}
	value >>= 1
	return [3]int{int(value>>26&0x1fff) + 1000, int(value>>13&0x1fff) + 1000, int(value&0x1fff) + 1000}, nil
}
func (s *SASVerification) mac(value, keyID string, peer bool) (string, error) {
	if s.shared == nil {
		return "", fmt.Errorf("SAS shared secret missing")
	}
	senderUser, senderDevice, receiverUser, receiverDevice := s.ownUser, s.ownDevice, s.peerUser, s.peerDevice
	if peer {
		senderUser, senderDevice, receiverUser, receiverDevice = s.peerUser, s.peerDevice, s.ownUser, s.ownDevice
	}
	info := "MATRIX_KEY_VERIFICATION_MAC" + senderUser + senderDevice + receiverUser + receiverDevice + s.transaction + keyID
	key, err := hkdf.Key(sha256.New, s.shared, nil, info, 32)
	if err != nil {
		return "", err
	}
	return EncodeBase64(hmacSHA256Sum(key, []byte(value))), nil
}
func (s *SASVerification) MACContent(signingKey string) (map[string]any, error) {
	if s.state != "key_received" {
		return nil, fmt.Errorf("unexpected SAS MAC state")
	}
	keyID := "ed25519:" + s.ownDevice
	mac, err := s.mac(signingKey, keyID, false)
	if err != nil {
		return nil, err
	}
	keys, err := s.mac(keyID, "KEY_IDS", false)
	if err != nil {
		return nil, err
	}
	s.state = "mac_sent"
	return map[string]any{"transaction_id": s.transaction, "mac": map[string]string{keyID: mac}, "keys": keys}, nil
}
func (s *SASVerification) VerifyPeerMAC(content map[string]any, peerKeys map[string]string) error {
	if s.state != "mac_sent" || content["transaction_id"] != s.transaction {
		return fmt.Errorf("unexpected SAS MAC transaction")
	}
	macs, ok := content["mac"].(map[string]any)
	if !ok || len(macs) == 0 || len(macs) > 10 {
		return fmt.Errorf("invalid SAS key map")
	}
	ids := make([]string, 0, len(macs))
	for key, value := range macs {
		mac, ok := value.(string)
		if !ok || !sasField(key) || !sasField(mac) {
			return fmt.Errorf("invalid SAS key MAC")
		}
		ids = append(ids, key)
	}
	if _, ok := macs["ed25519:"+s.peerDevice]; !ok {
		return fmt.Errorf("SAS peer device key missing")
	}
	sort.Strings(ids)
	keys, _ := content["keys"].(string)
	expected, err := s.mac(strings.Join(ids, ","), "KEY_IDS", true)
	if err != nil || !sasField(keys) || !hmac.Equal([]byte(keys), []byte(expected)) {
		return fmt.Errorf("SAS key-list MAC mismatch")
	}
	for _, id := range ids {
		key := peerKeys[id]
		if !sasField(key) {
			return fmt.Errorf("unknown SAS peer key")
		}
		expected, err := s.mac(key, id, true)
		if err != nil || !hmac.Equal([]byte(macs[id].(string)), []byte(expected)) {
			return fmt.Errorf("SAS device-key MAC mismatch")
		}
	}
	s.state = "verified"
	return nil
}
func (s *SASVerification) Verified() bool { return s.state == "verified" }

func decodeSASProposal(raw []byte) (map[string]any, error) {
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&result)
	return result, err
}
