package testutil

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
)

func CapturePythonRequests(t *testing.T, url, body, title string) []notify.RequestSpec {
	t.Helper()

	script := filepath.Join(RepoRoot(t), "internal", "testutil", "scripts", "capture_request.py")
	stdout, stderr, err := RunPythonScript(t, script, "--url", url, "--body", body, "--title", title)
	if err != nil {
		t.Fatalf("capture request failed: %v (stderr: %s)", err, strings.TrimSpace(stderr))
	}

	var specs []notify.RequestSpec
	if err := json.Unmarshal([]byte(stdout), &specs); err != nil {
		t.Fatalf("parse request specs: %v (output: %s)", err, strings.TrimSpace(stdout))
	}

	return specs
}

func CapturePythonRequestsWithType(t *testing.T, url, body, title string, notifyType notify.NotifyType) []notify.RequestSpec {
	t.Helper()

	script := filepath.Join(RepoRoot(t), "internal", "testutil", "scripts", "capture_request.py")
	stdout, stderr, err := RunPythonScript(
		t,
		script,
		"--url", url,
		"--body", body,
		"--title", title,
		"--type", string(notifyType),
	)
	if err != nil {
		t.Fatalf("capture request failed: %v (stderr: %s)", err, strings.TrimSpace(stderr))
	}

	var specs []notify.RequestSpec
	if err := json.Unmarshal([]byte(stdout), &specs); err != nil {
		t.Fatalf("parse request specs: %v (output: %s)", err, strings.TrimSpace(stdout))
	}

	return specs
}
