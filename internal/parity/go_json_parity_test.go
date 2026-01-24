package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/cli"
	"github.com/unraid/apprise-go/internal/testutil"
	"github.com/unraid/apprise-go/internal/testutil/capture"
)

func TestGoJSONMatchesPython(t *testing.T) {
	pythonSrv := capture.NewServer(t)
	defer pythonSrv.Close()
	goSrv := capture.NewServer(t)
	defer goSrv.Close()

	pythonReq := sendPythonJSON(t, pythonSrv)
	goReq := sendGoJSON(t, goSrv)

	if pythonReq.Method != goReq.Method {
		t.Fatalf("method mismatch: python=%s go=%s", pythonReq.Method, goReq.Method)
	}
	if pythonReq.Path != goReq.Path {
		t.Fatalf("path mismatch: python=%s go=%s", pythonReq.Path, goReq.Path)
	}

	pythonContentType := pythonReq.Header.Get("Content-Type")
	goContentType := goReq.Header.Get("Content-Type")
	if pythonContentType != goContentType {
		t.Fatalf("content-type mismatch: python=%s go=%s", pythonContentType, goContentType)
	}

	pythonUserAgent := pythonReq.Header.Get("User-Agent")
	goUserAgent := goReq.Header.Get("User-Agent")
	if pythonUserAgent != goUserAgent {
		t.Fatalf("user-agent mismatch: python=%s go=%s", pythonUserAgent, goUserAgent)
	}

	pythonPayload := decodeJSON(t, pythonReq.Body)
	goPayload := decodeJSON(t, goReq.Body)
	if !reflect.DeepEqual(pythonPayload, goPayload) {
		t.Fatalf("payload mismatch: python=%v go=%v", pythonPayload, goPayload)
	}
}

func sendPythonJSON(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	target := buildTarget(t, srv.URL())
	args := []string{
		"--body", "hello from python",
		"--title", "apprise parity",
		"--disable-async",
		target,
	}

	stdout, stderr, err := testutil.RunApprise(t, args...)
	if err != nil {
		t.Fatalf("python apprise failed: %v (stdout: %s, stderr: %s)", err, strings.TrimSpace(stdout), strings.TrimSpace(stderr))
	}

	return singleRequest(t, srv)
}

func sendGoJSON(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	target := buildTarget(t, srv.URL())
	args := []string{
		"--body", "hello from python",
		"--title", "apprise parity",
		"--disable-async",
		target,
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("go apprise failed: code=%d stdout=%s stderr=%s", code, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}

	return singleRequest(t, srv)
}

func singleRequest(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	requests := srv.Requests()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}

	return requests[0]
}

func buildTarget(t *testing.T, rawURL string) string {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return fmt.Sprintf("json://%s/notify", u.Host)
}

func decodeJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()

	if len(bytes.TrimSpace(data)) == 0 {
		t.Fatalf("empty json payload")
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode json payload: %v", err)
	}

	return payload
}

func assertRequestBasics(t *testing.T, req capture.Request) {
	t.Helper()

	if req.Method != http.MethodPost {
		t.Fatalf("expected POST request, got %s", req.Method)
	}

	if req.Path != "/notify" {
		t.Fatalf("expected path /notify, got %s", req.Path)
	}
}
