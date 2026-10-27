package notify

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestNextcloudTalkBotSignature(t *testing.T) {
	// Independently generated with Python hmac.new(secret, nonce+message, sha256).
	nonce := strings.Repeat("ab", 32)
	got := nextcloudTalkMAC("public-fixture-secret", nonce, "Title\r\nBody")
	if got != "99db176857fb429e669b6c398b2e851b519d09e1313702b93eb80e8363f673ab" {
		t.Fatalf("signature %s", got)
	}
	parsed, err := ParseURL("nctalks://example.com/room?secret=public-fixture-secret&silent=yes")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewNextcloudTalkTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := target.BuildRequest("Body", "Title", NotifyInfo)
	if err != nil {
		t.Fatal(err)
	}
	if spec.URL != "https://example.com/ocs/v2.php/apps/spreed/api/v1/bot/room/message" || spec.Headers["Authorization"] != "" {
		t.Fatalf("bot routing/auth %+v", spec)
	}
	random := spec.Headers["X-Nextcloud-Talk-Bot-Random"]
	decoded, err := hex.DecodeString(random)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("nonce %q", random)
	}
	if spec.Headers["X-Nextcloud-Talk-Bot-Signature"] != nextcloudTalkMAC("public-fixture-secret", random, "Title\r\nBody") {
		t.Fatal("signature does not cover nonce and message")
	}
	var payload map[string]any
	if json.Unmarshal([]byte(spec.Body), &payload) != nil || payload["silent"] != true {
		t.Fatalf("payload %s", spec.Body)
	}
}
