package parity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

func TestMatrixSASIndependentPeer(t *testing.T) {
	python := testutil.PythonPath(t)
	_ = python
	script := filepath.Join(testutil.RepoRoot(t), "internal/testutil/scripts/matrix_sas_peer.py")
	oracle := func(input map[string]any) map[string]any {
		t.Helper()
		data, _ := json.Marshal(input)
		out, stderr, err := testutil.RunPythonScript(t, script, string(data))
		if err != nil {
			t.Fatalf("independent SAS peer: %v %s", err, stderr)
		}
		var result map[string]any
		if err = json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	keys := oracle(map[string]any{"operation": "keys"})
	for _, tamper := range []bool{false, true} {
		t.Run(fmt.Sprintf("tamper-%t", tamper), func(t *testing.T) {
			notify.ConfigureStorage(t.TempDir(), 8, nil)
			defer notify.ConfigureStorage("", 8, nil)
			const user = "@same:example.com"
			start := map[string]any{"transaction_id": "sas-transaction", "from_device": "PEER", "method": "m.sas.v1", "key_agreement_protocols": []string{"curve25519-hkdf-sha256"}, "hashes": []string{"sha256"}, "message_authentication_codes": []string{"hkdf-hmac-sha256.v2"}, "short_authentication_string": []string{"decimal"}}
			var mu sync.Mutex
			var uploaded, accept, ownKey, ownMAC map[string]any
			polls, messages, cancels := 0, 0, 0
			event := func(kind string, content map[string]any) map[string]any {
				return map[string]any{"sender": user, "type": "m.key.verification." + kind, "content": content}
			}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				respond := func(value any) {
					if err := json.NewEncoder(w).Encode(value); err != nil {
						t.Error(err)
					}
				}
				var payload map[string]any
				if r.Body != nil && r.Method != http.MethodGet {
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/account/whoami"):
					respond(map[string]any{"user_id": user, "device_id": "SENDER"})
				case strings.HasSuffix(r.URL.Path, "/keys/upload"):
					uploaded = payload["device_keys"].(map[string]any)
					respond(map[string]any{})
				case strings.HasSuffix(r.URL.Path, "/keys/query"):
					respond(map[string]any{"device_keys": map[string]any{user: map[string]any{"PEER": keys["device"]}}})
				case strings.HasSuffix(r.URL.Path, "/sync"):
					polls++
					var events []any
					switch polls {
					case 1:
						events = []any{event("request", map[string]any{"transaction_id": "sas-transaction", "from_device": "PEER", "methods": []string{"m.sas.v1"}, "timestamp": time.Now().UnixMilli()})}
					case 2:
						events = []any{event("start", start)}
					case 3:
						events = []any{event("key", map[string]any{"transaction_id": "sas-transaction", "key": keys["public_key"]})}
					case 4:
						mac := oracle(map[string]any{"operation": "verify", "start": start, "accept": accept, "key": ownKey, "mac": ownMAC, "device_keys": uploaded})
						if tamper {
							mac["keys"] = "tampered"
						}
						events = []any{event("mac", mac), event("done", map[string]any{"transaction_id": "sas-transaction"})}
					default:
						t.Error("unexpected verification poll")
					}
					respond(map[string]any{"next_batch": fmt.Sprint(polls), "to_device": map[string]any{"events": events}})
				case strings.Contains(r.URL.Path, "/sendToDevice/"):
					content := payload["messages"].(map[string]any)[user].(map[string]any)["PEER"].(map[string]any)
					switch {
					case strings.Contains(r.URL.Path, ".accept/"):
						accept = content
					case strings.Contains(r.URL.Path, ".key/"):
						ownKey = content
					case strings.Contains(r.URL.Path, ".mac/"):
						ownMAC = content
					case strings.Contains(r.URL.Path, ".cancel/"):
						cancels++
					}
					respond(map[string]any{})
				case strings.Contains(r.URL.Path, "/join/"):
					respond(map[string]any{"room_id": "!room:example.com"})
				case strings.HasSuffix(r.URL.Path, "/state/m.room.encryption"):
					w.WriteHeader(404)
					respond(map[string]any{})
				case strings.Contains(r.URL.Path, "/send/m.room.message/"):
					messages++
					respond(map[string]any{"event_id": "$message"})
				default:
					t.Errorf("unexpected SAS request %s", r.URL.Path)
					w.WriteHeader(404)
					respond(map[string]any{})
				}
			}))
			defer server.Close()
			previous := http.DefaultClient
			http.DefaultClient = server.Client()
			defer func() { http.DefaultClient = previous }()
			parsed, err := notify.ParseURL("matrixs://token@" + strings.TrimPrefix(server.URL, "https://") + "/!room?autoverify=yes&discovery=no&image=no")
			if err != nil {
				t.Fatal(err)
			}
			target, err := notify.NewMatrixTarget(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if err = notify.DispatchSendWithInput(target, parsed, "body", "title", "", notify.NotifyInfo, nil); err != nil {
				t.Fatal(err)
			}
			_, trusted := notify.StoreFor(parsed).Get("e2ee_verified_binding")
			if trusted == tamper {
				t.Fatalf("trusted=%t tamper=%t", trusted, tamper)
			}
			if err = notify.DispatchSendWithInput(target, parsed, "next", "title", "", notify.NotifyInfo, nil); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if polls != 4 || messages != 2 || cancels != map[bool]int{false: 0, true: 1}[tamper] {
				t.Fatalf("polls%d messages%d cancels%d", polls, messages, cancels)
			}
		})
	}
}
