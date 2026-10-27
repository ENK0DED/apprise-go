package notify

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type wpushResponseTransport struct {
	status int
	body   string
}

func (r wpushResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: r.status, Body: io.NopCloser(strings.NewReader(r.body)), Header: make(http.Header), Request: req}, nil
}

func TestWPushResponseContract(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	parsed, err := ParseURL("wpush://WPUSH0123456789abcdefghijklmnopq")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewWPushTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body string
		status     int
		success    bool
	}{
		{"zero", `{"code":0}`, 200, true},
		{"float-zero", `{"code":0.0}`, 200, true},
		{"boolean", `{"code":false}`, 200, false},
		{"string", `{"code":"0"}`, 200, false},
		{"missing", `{}`, 200, false},
		{"error", `{"code":1}`, 200, false},
		{"malformed", `ok`, 200, false},
		{"http-error", `{"code":0}`, 500, false},
		{"other-success", `{"code":0}`, 201, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			http.DefaultTransport = wpushResponseTransport{tc.status, tc.body}
			if err := target.Send("body", "title", NotifyInfo); (err == nil) != tc.success {
				t.Fatalf("success=%v, error=%v", tc.success, err)
			}
		})
	}
}

func TestWPushRejectsInvalidConfiguration(t *testing.T) {
	for _, raw := range []string{
		"wpush://WPUSHshort",
		"wpush://WPUSH0123456789abcdefghijklmnopq?channel=unknown",
		"wpush://WPUSH0123456789abcdefghijklmnopq/topic?group=group",
	} {
		parsed, err := ParseURL(raw)
		if err != nil {
			continue
		}
		if _, err := NewWPushTarget(parsed); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
