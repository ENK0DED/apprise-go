package parity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/testutil"
	"github.com/unraid/apprise-go/internal/testutil/capture"
)

func TestPythonJSONCapture(t *testing.T) {
	srv := capture.NewServer(t)
	defer srv.Close()

	u, err := url.Parse(srv.URL())
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	target := fmt.Sprintf("json://%s/notify", u.Host)
	args := []string{
		"--body", "hello from python",
		"--title", "apprise parity",
		"--disable-async",
		target,
	}

	stdout, stderr, err := testutil.RunApprise(t, args...)
	if err != nil {
		t.Fatalf("apprise CLI failed: %v (stdout: %s, stderr: %s)", err, strings.TrimSpace(stdout), strings.TrimSpace(stderr))
	}

	requests := srv.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	req := requests[0]
	if req.Method != http.MethodPost {
		t.Fatalf("expected POST request, got %s", req.Method)
	}

	if req.Path != "/notify" {
		t.Fatalf("expected path /notify, got %s", req.Path)
	}

	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", contentType)
	}

	var payload map[string]any
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		t.Fatalf("parse json payload: %v", err)
	}

	assertPayloadString(t, payload, "version", "1.0")
	assertPayloadString(t, payload, "title", "apprise parity")
	assertPayloadString(t, payload, "message", "hello from python")
	assertPayloadString(t, payload, "type", "info")

	attachments, ok := payload["attachments"].([]any)
	if !ok {
		t.Fatalf("expected attachments to be a list, got %T", payload["attachments"])
	}
	if len(attachments) != 0 {
		t.Fatalf("expected no attachments, got %d", len(attachments))
	}
}

func assertPayloadString(t *testing.T, payload map[string]any, key, expected string) {
	t.Helper()

	value, ok := payload[key]
	if !ok {
		t.Fatalf("payload missing %s", key)
	}

	actual, ok := value.(string)
	if !ok {
		t.Fatalf("payload %s not a string: %T", key, value)
	}

	if actual != expected {
		t.Fatalf("payload %s mismatch: got %s want %s", key, actual, expected)
	}
}
