package notify

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func simplePushResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestSimplePushRejectsMissingSaltAndSendScope(t *testing.T) {
	for _, tc := range []struct{ url, response string }{{"spush://password@api-token", `{}`}, {"spush://spi_credential.AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8/@member", `{"scopes":["read"]}`}} {
		t.Run(tc.url, func(t *testing.T) {
			calls := 0
			old := http.DefaultTransport
			http.DefaultTransport = reviewTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" {
					t.Error("invalid crypto setup created a task")
				}
				return simplePushResponse(200, tc.response), nil
			})
			defer func() { http.DefaultTransport = old }()
			parsed, err := ParseURL(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			target, err := NewSimplePushTarget(parsed)
			if err != nil {
				t.Fatal(err)
			}
			if err = DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, nil); err == nil {
				t.Fatal("invalid crypto setup succeeded")
			}
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}
func TestSimplePushPartialRetryAndFailedUpload(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	calls := map[string]int{}
	http.DefaultTransport = reviewTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/v1/tasks/json" {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				return nil, err
			}
			topic, _ := payload["topic"].(string)
			calls[topic]++
			if topic == "first" && calls[topic] == 1 {
				return simplePushResponse(503, `{}`), nil
			}
			return simplePushResponse(201, `{}`), nil
		}
		return nil, fmt.Errorf("unexpected URL %s", r.URL)
	})
	parsed, err := ParseURL("spush://api-token/first/second?retry=1&wait=0")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewSimplePushTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if err = DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, nil); err != nil {
		t.Fatal(err)
	}
	if calls["first"] != 2 || calls["second"] != 1 {
		t.Fatalf("calls %+v", calls)
	}
	var steps []string
	http.DefaultTransport = reviewTransport(func(r *http.Request) (*http.Response, error) {
		steps = append(steps, r.URL.Path)
		switch r.URL.Path {
		case "/v1/tasks/json":
			return simplePushResponse(201, `{"attachments":[{"id":"attachment-1"}]}`), nil
		case "/v1/attachments/attachment-1/upload-url":
			return simplePushResponse(500, `{}`), nil
		case "/v1/attachments/attachment-1/failed":
			return simplePushResponse(204, ``), nil
		default:
			return nil, fmt.Errorf("unexpected URL %s", r.URL)
		}
	})
	parsed, err = ParseURL("spush://api-token")
	if err != nil {
		t.Fatal(err)
	}
	target, err = NewSimplePushTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	err = DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, []Attachment{{Name: "notes.txt", Data: []byte("content")}})
	if err == nil {
		t.Fatal("failed upload succeeded")
	}
	if strings.Join(steps, ",") != "/v1/tasks/json,/v1/attachments/attachment-1/upload-url,/v1/attachments/attachment-1/failed" {
		t.Fatalf("upload lifecycle %+v", steps)
	}
}
