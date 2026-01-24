package parity

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/cli"
	"github.com/unraid/apprise-go/internal/testutil"
	"github.com/unraid/apprise-go/internal/testutil/capture"
)

func TestGoFormMatchesPython(t *testing.T) {
	pythonSrv := capture.NewServer(t)
	defer pythonSrv.Close()
	goSrv := capture.NewServer(t)
	defer goSrv.Close()

	pythonReq := sendPythonForm(t, pythonSrv)
	goReq := sendGoForm(t, goSrv)

	if pythonReq.Method != goReq.Method {
		t.Fatalf("method mismatch: python=%s go=%s", pythonReq.Method, goReq.Method)
	}
	if pythonReq.Path != goReq.Path {
		t.Fatalf("path mismatch: python=%s go=%s", pythonReq.Path, goReq.Path)
	}

	if contentTypeBase(pythonReq.Header.Get("Content-Type")) != contentTypeBase(goReq.Header.Get("Content-Type")) {
		t.Fatalf("content-type mismatch: python=%s go=%s", pythonReq.Header.Get("Content-Type"), goReq.Header.Get("Content-Type"))
	}

	if pythonReq.Header.Get("User-Agent") != goReq.Header.Get("User-Agent") {
		t.Fatalf("user-agent mismatch: python=%s go=%s", pythonReq.Header.Get("User-Agent"), goReq.Header.Get("User-Agent"))
	}

	pythonValues := parseFormBody(t, pythonReq)
	goValues := parseFormBody(t, goReq)

	if pythonValues.Encode() != goValues.Encode() {
		t.Fatalf("form payload mismatch: python=%s go=%s", pythonValues.Encode(), goValues.Encode())
	}
}

func sendPythonForm(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	target := buildTargetWithScheme(t, srv.URL(), "form")
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

func sendGoForm(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	target := buildTargetWithScheme(t, srv.URL(), "form")
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

func parseFormBody(t *testing.T, req capture.Request) url.Values {
	t.Helper()

	values, err := url.ParseQuery(string(req.Body))
	if err != nil {
		t.Fatalf("parse form body: %v", err)
	}

	return values
}

func buildTargetWithScheme(t *testing.T, rawURL, scheme string) string {
	t.Helper()

	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return fmt.Sprintf("%s://%s/notify", scheme, u.Host)
}

func contentTypeBase(value string) string {
	parts := strings.Split(value, ";")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
