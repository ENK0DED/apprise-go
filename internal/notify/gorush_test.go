package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type gorushRetryTransport struct {
	first  string
	tokens [][]string
}

func (r *gorushRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body struct {
		Notifications []struct {
			Tokens []string `json:"tokens"`
		} `json:"notifications"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return nil, err
	}
	r.tokens = append(r.tokens, body.Notifications[0].Tokens)
	response := `{}`
	if len(r.tokens) == 1 {
		response = r.first
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header), Request: req}, nil
}

func TestGorushRetriesOnlyExactlyIdentifiedFailures(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, tc := range []struct {
		name, response string
		want           int
	}{
		{"exact", `{"logs":[{"type":"failed-push","token":"beta"}]}`, 1},
		{"masked", `{"logs":[{"type":"failed-push","token":"b***"}]}`, 2},
		{"missing", `{"logs":[{"type":"failed-push"}]}`, 2},
		{"wrong-type", `{"logs":[{"type":"failed-push","token":123}]}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &gorushRetryTransport{first: tc.response}
			http.DefaultTransport = transport
			parsed, err := ParseURL("gorush://example.com/alpha/beta?retry=1")
			if err != nil {
				t.Fatal(err)
			}
			target, err := NewGorushTarget(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if err := DispatchSendWithOverflow(target, parsed, "body", "title", NotifyInfo, nil); err != nil {
				t.Fatal(err)
			}
			if len(transport.tokens) != 2 || len(transport.tokens[1]) != tc.want {
				t.Fatalf("request targets: %v", transport.tokens)
			}
			if tc.want == 1 && transport.tokens[1][0] != "beta" {
				t.Fatalf("retried wrong token: %v", transport.tokens)
			}
			if err := DispatchSendWithOverflow(target, parsed, "next", "title", NotifyInfo, nil); err != nil {
				t.Fatal(err)
			}
			if len(transport.tokens) != 3 || len(transport.tokens[2]) != 2 {
				t.Fatalf("next notification inherited delivery state: %v", transport.tokens)
			}
		})
	}
}
