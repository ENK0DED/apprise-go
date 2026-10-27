package notify

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
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

// wpushRedirectTransport routes only the initial service request to a local
// server. Redirects use their actual destination, so forwarding is observable.
type wpushRedirectTransport struct {
	initial *url.URL
	base    *http.Transport
}

func (r wpushRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.wpush.cn" {
		req = req.Clone(req.Context())
		copyURL := *req.URL
		copyURL.Scheme, copyURL.Host = r.initial.Scheme, r.initial.Host
		req.URL = &copyURL
	}
	return r.base.RoundTrip(req)
}

func TestWPushDoesNotForwardCredentialsOnRedirect(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var forwarded atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				forwarded.Add(1)
				_, _ = io.WriteString(w, `{"code":0}`)
			}))
			defer destination.Close()
			initial := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", destination.URL)
				w.WriteHeader(status)
			}))
			defer initial.Close()
			initialURL, err := url.Parse(initial.URL)
			if err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{}
			defer transport.CloseIdleConnections()
			http.DefaultClient = &http.Client{Transport: wpushRedirectTransport{initialURL, transport}}
			parsed, err := ParseURL("wpush://WPUSH0123456789abcdefghijklmnopq")
			if err != nil {
				t.Fatal(err)
			}
			target, err := NewWPushTarget(parsed)
			if err != nil {
				t.Fatal(err)
			}
			err = target.Send("private body", "private title", NotifyInfo)
			if forwarded.Load() != 0 {
				t.Fatalf("forwarded credentials to redirect destination %d times", forwarded.Load())
			}
			if err == nil {
				t.Fatal("accepted redirect as a successful notification")
			}
		})
	}
}
