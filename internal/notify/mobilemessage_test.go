package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mobileMessageRetryTransport struct {
	first string
	keys  []string
	sizes []int
}

func (r *mobileMessageRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body struct {
		Messages []any `json:"messages"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return nil, err
	}
	r.keys = append(r.keys, req.Header.Get("Idempotency-Key"))
	r.sizes = append(r.sizes, len(body.Messages))
	response := r.first
	if len(r.keys) > 1 {
		results := make([]map[string]string, len(body.Messages))
		for i := range results {
			results[i] = map[string]string{"status": "success"}
		}
		encoded, _ := json.Marshal(map[string]any{"status": "complete", "results": results})
		response = string(encoded)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header), Request: req}, nil
}

func TestMobileMessageRetryIdentityAndRecipients(t *testing.T) {
	t.Setenv("APPRISE_FIXED_TIME", "")
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, tc := range []struct {
		name, response string
		wantSize       int
		reuse          bool
	}{
		{"unanswered", `{}`, 2, true},
		{"partial", `{"status":"complete","results":[{"status":"success"},{"status":"failed"}]}`, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &mobileMessageRetryTransport{first: tc.response}
			http.DefaultTransport = transport
			parsed, err := ParseURL("mobilemessage://user:pass@Unraid/0412345678/0498765432?retry=1")
			if err != nil {
				t.Fatal(err)
			}
			target, err := NewMobileMessageTarget(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if err := DispatchSendWithOverflow(target, parsed, "body", "title", NotifyInfo, nil); err != nil {
				t.Fatal(err)
			}
			if len(transport.keys) != 2 || transport.sizes[1] != tc.wantSize {
				t.Fatalf("batch sizes=%v", transport.sizes)
			}
			if same := transport.keys[0] == transport.keys[1]; same != tc.reuse {
				t.Fatalf("key reuse=%v want %v", same, tc.reuse)
			}
			if transport.keys[0] == "" {
				t.Fatal("missing idempotency key")
			}
			if err := DispatchSendWithOverflow(target, parsed, "body", "title", NotifyInfo, nil); err != nil {
				t.Fatal(err)
			}
			if len(transport.keys) != 3 || transport.sizes[2] != 2 || transport.keys[2] == transport.keys[0] {
				t.Fatal("next notification retained delivery or idempotency state")
			}
		})
	}
}
