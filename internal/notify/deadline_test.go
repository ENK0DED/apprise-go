package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotificationDeadlineCoversRetriesAndResponseBody(t *testing.T) {
	for _, bodyStall := range []bool{false, true} {
		t.Run(map[bool]string{false: "retry-budget", true: "response-body"}[bodyStall], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if bodyStall {
					w.Header().Set("Content-Length", "100")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				w.WriteHeader(503)
			}))
			defer server.Close()
			parsed, err := ParseURL("json://" + strings.TrimPrefix(server.URL, "http://") + "?retry=5&wait=0.1")
			if err != nil {
				t.Fatal(err)
			}
			var target Sender
			if bodyStall {
				target = deadlineReadSender(server.URL)
			} else {
				target, err = NewJSONTarget(parsed)
			}
			if err != nil {
				t.Fatal(err)
			}
			SetNotificationDeadline(parsed, time.Now().Add(150*time.Millisecond))
			start := time.Now()
			err = DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, nil)
			if err == nil {
				t.Fatal("deadline unexpectedly succeeded")
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("deadline took %s", elapsed)
			}
			if calls.Load() > 2 {
				t.Fatalf("retried beyond budget: %d", calls.Load())
			}
		})
	}
}

type deadlineReadSender string

func (s deadlineReadSender) Send(body, title string, kind NotifyType) error {
	request, err := http.NewRequest(http.MethodPost, string(s), strings.NewReader(body))
	if err != nil {
		return err
	}
	response, err := httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.ReadAll(response.Body)
	return err
}
