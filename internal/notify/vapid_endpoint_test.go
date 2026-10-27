package notify

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVapidEndpointOrigins(t *testing.T) {
	for _, tc := range []struct{ url, origin string }{{"https://PUSH.example:443/a", "https://push.example"}, {"https://push.example:8443/a", "https://push.example:8443"}, {"https://[::1]:8443/a", "https://[::1]:8443"}, {"http://push.example/a", ""}, {"https://user:pass@push.example/a", ""}, {"https:///a", ""}, {"https://push.example:99999/a", ""}} {
		got, err := vapidOrigin(tc.url)
		if got != tc.origin || (err != nil) != (tc.origin == "") {
			t.Fatalf("%s got %q %v", tc.url, got, err)
		}
	}
}
func TestVapidJWTAudienceIsEndpointOrigin(t *testing.T) {
	key, _, err := loadVapidKey("../testutil/fixtures/vapid_test_key.pem")
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://push.example", "https://push.example:8443"} {
		token, err := buildVapidJWT(key, "user@example.com", origin)
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
		if err != nil {
			t.Fatal(err)
		}
		var claims map[string]any
		if json.Unmarshal(data, &claims) != nil || claims["aud"] != origin {
			t.Fatalf("claims %s", data)
		}
	}
}
func TestVapidPartialRetryAndRetirement(t *testing.T) {
	calls := map[string]int{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/gone":
			w.WriteHeader(410)
		case "/retry":
			if calls[r.URL.Path] == 1 {
				w.WriteHeader(500)
				return
			}
			w.WriteHeader(202)
		default:
			w.WriteHeader(201)
		}
	}))
	defer server.Close()
	old := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = old }()
	t.Setenv("APPRISE_VAPID_TEST_JWT", "public-fixture-token")
	t.Setenv("APPRISE_VAPID_TEST_PUBLIC_KEY", "public-fixture-key")
	t.Setenv("APPRISE_VAPID_TEST_ENCRYPTED", "Ym9keQ==")
	original, err := os.ReadFile("../parity/providers/vapid/subscriptions.json")
	if err != nil {
		t.Fatal(err)
	}
	var source map[string]vapidSubscriptionFile
	if err = json.Unmarshal(original, &source); err != nil {
		t.Fatal(err)
	}
	entries := map[string]vapidSubscriptionFile{}
	for _, name := range []string{"ok", "gone", "retry"} {
		entry := source["user@example.com"]
		entry.Endpoint = server.URL + "/" + name
		entries[name] = entry
	}
	data, _ := json.Marshal(entries)
	file := filepath.Join(t.TempDir(), "subscriptions.json")
	if err = os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseURL("vapid://user@example.com/ok/gone/retry?subfile=" + file + "&retry=1&wait=0&store=no")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewVapidTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if err = DispatchSendWithInput(target, parsed, "body", "", "", NotifyInfo, nil); err != nil {
		t.Fatal(err)
	}
	if calls["/ok"] != 1 || calls["/gone"] != 1 || calls["/retry"] != 2 {
		t.Fatalf("calls %+v", calls)
	}
	pruned, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var remaining map[string]any
	if json.Unmarshal(pruned, &remaining) != nil || remaining["gone"] != nil || remaining["ok"] == nil || remaining["retry"] == nil {
		t.Fatalf("pruned %s", pruned)
	}
}
