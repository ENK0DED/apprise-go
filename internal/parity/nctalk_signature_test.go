package parity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
)

// Random nonces are compared through their protocol contract, independently
// for both senders. Only verified signatures may be omitted from byte parity.
func verifyNextcloudBotRequest(spec notify.RequestSpec, secret string) error {
	header := func(name string) string {
		for key, value := range spec.Headers {
			if strings.EqualFold(key, name) {
				return value
			}
		}
		return ""
	}
	nonce := header("X-Nextcloud-Talk-Bot-Random")
	decoded, err := hex.DecodeString(nonce)
	if err != nil || len(decoded) != 32 || nonce != strings.ToLower(nonce) {
		return fmt.Errorf("invalid Nextcloud bot nonce")
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err = json.Unmarshal([]byte(spec.Body), &payload); err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(nonce + payload.Message))
	signature, err := hex.DecodeString(header("X-Nextcloud-Talk-Bot-Signature"))
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return fmt.Errorf("Nextcloud bot signature does not authenticate its message")
	}
	return nil
}
func nextcloudBotVolatileHeaders(t *testing.T, parsed *notify.ParsedURL, expected, got []notify.RequestSpec, headers []string) []string {
	t.Helper()
	if parsed == nil || parsed.Query["secret"] == "" || (parsed.Scheme != "nctalk" && parsed.Scheme != "nctalks") {
		return headers
	}
	for _, sequence := range [][]notify.RequestSpec{expected, got} {
		for _, spec := range sequence {
			if err := verifyNextcloudBotRequest(spec, parsed.Query["secret"]); err != nil {
				t.Fatal(err)
			}
		}
	}
	result := append([]string{}, headers...)
	return append(result, "X-Nextcloud-Talk-Bot-Random", "X-Nextcloud-Talk-Bot-Signature")
}
func TestNextcloudBotReceiptRejectsTampering(t *testing.T) {
	spec := notify.RequestSpec{Headers: map[string]string{"X-Nextcloud-Talk-Bot-Random": strings.Repeat("00", 32)}, Body: `{"message":"original"}`}
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte(strings.Repeat("00", 32) + "original"))
	spec.Headers["X-Nextcloud-Talk-Bot-Signature"] = hex.EncodeToString(mac.Sum(nil))
	if err := verifyNextcloudBotRequest(spec, "secret"); err != nil {
		t.Fatal(err)
	}
	spec.Body = `{"message":"tampered"}`
	if err := verifyNextcloudBotRequest(spec, "secret"); err == nil {
		t.Fatal("tampered message passed signature receipt")
	}
}
