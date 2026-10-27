package notify

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMatrixRetryPreservesAcceptedRooms(t *testing.T) {
	var first, second atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/account/whoami"):
			fmt.Fprint(w, `{"user_id":"@sender:example.com","device_id":"DEVICE"}`)
		case strings.Contains(r.URL.Path, "/join/"):
			room := "first"
			if strings.Contains(r.URL.Path, "second") {
				room = "second"
			}
			fmt.Fprintf(w, `{"room_id":"!%s:example.com"}`, room)
		case strings.Contains(r.URL.Path, "/rooms/!first:"):
			first.Add(1)
			fmt.Fprint(w, `{"event_id":"first-event"}`)
		case strings.Contains(r.URL.Path, "/rooms/!second:"):
			if second.Add(1) == 1 {
				w.WriteHeader(500)
			} else {
				fmt.Fprint(w, `{"event_id":"second-event"}`)
			}
		default:
			t.Errorf("unexpected Matrix path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	parsed, err := ParseURL("matrix://:token@" + strings.TrimPrefix(server.URL, "http://") + "/%23first/%23second?e2ee=no&discovery=no&retry=1&wait=0&store=no")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewMatrixTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if err = DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, nil); err != nil {
		t.Fatal(err)
	}
	if first.Load() != 1 || second.Load() != 2 {
		t.Fatalf("delivery counts first=%d second=%d", first.Load(), second.Load())
	}
	if err = DispatchSendWithInput(target, parsed, "new body", "title", "", NotifyInfo, nil); err != nil {
		t.Fatal(err)
	}
	if first.Load() != 2 || second.Load() != 3 {
		t.Fatalf("next notification retained deliveries: first=%d second=%d", first.Load(), second.Load())
	}
}
