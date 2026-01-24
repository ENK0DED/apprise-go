package parity

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/cli"
	"github.com/unraid/apprise-go/internal/testutil"
	"github.com/unraid/apprise-go/internal/testutil/capture"
)

func TestGoXMLMatchesPython(t *testing.T) {
	pythonSrv := capture.NewServer(t)
	defer pythonSrv.Close()
	goSrv := capture.NewServer(t)
	defer goSrv.Close()

	pythonReq := sendPythonXML(t, pythonSrv)
	goReq := sendGoXML(t, goSrv)

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

	assertXMLPayloadMatches(t, string(pythonReq.Body), string(goReq.Body))
}

func sendPythonXML(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	target := buildTargetWithScheme(t, srv.URL(), "xml")
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

func sendGoXML(t *testing.T, srv *capture.Server) capture.Request {
	t.Helper()

	target := buildTargetWithScheme(t, srv.URL(), "xml")
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

func assertXMLPayloadMatches(t *testing.T, pythonPayload, goPayload string) {
	t.Helper()

	python := extractXMLFields(t, pythonPayload)
	goData := extractXMLFields(t, goPayload)

	if python["Version"] != goData["Version"] {
		t.Fatalf("xml version mismatch: python=%s go=%s", python["Version"], goData["Version"])
	}
	if python["Subject"] != goData["Subject"] {
		t.Fatalf("xml subject mismatch: python=%s go=%s", python["Subject"], goData["Subject"])
	}
	if python["Message"] != goData["Message"] {
		t.Fatalf("xml message mismatch: python=%s go=%s", python["Message"], goData["Message"])
	}
	if python["MessageType"] != goData["MessageType"] {
		t.Fatalf("xml type mismatch: python=%s go=%s", python["MessageType"], goData["MessageType"])
	}

	pythonXSD := extractXMLAttribute(t, pythonPayload, "xmlns:xsi")
	goXSD := extractXMLAttribute(t, goPayload, "xmlns:xsi")
	if pythonXSD != goXSD {
		t.Fatalf("xml xsd mismatch: python=%s go=%s", pythonXSD, goXSD)
	}
}

func extractXMLFields(t *testing.T, payload string) map[string]string {
	t.Helper()

	fields := map[string]string{}
	for _, tag := range []string{"Version", "Subject", "Message", "MessageType"} {
		value, ok := extractXMLTag(payload, tag)
		if !ok {
			t.Fatalf("missing xml tag %s", tag)
		}
		fields[tag] = value
	}
	return fields
}

func extractXMLTag(payload, tag string) (string, bool) {
	re := regexp.MustCompile(fmt.Sprintf(`<%s>(.*?)</%s>`, tag, tag))
	match := re.FindStringSubmatch(payload)
	if len(match) < 2 {
		return "", false
	}
	return match[1], true
}

func extractXMLAttribute(t *testing.T, payload, attribute string) string {
	t.Helper()

	re := regexp.MustCompile(fmt.Sprintf(`%s=\"([^\"]+)\"`, regexp.QuoteMeta(attribute)))
	match := re.FindStringSubmatch(payload)
	if len(match) < 2 {
		t.Fatalf("missing xml attribute %s", attribute)
	}
	return match[1]
}
