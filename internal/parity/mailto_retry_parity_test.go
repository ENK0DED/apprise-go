package parity

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

func TestSMTPPartialRetryParity(t *testing.T) {
	capture := testutil.StartSMTPCapture(t)
	defer capture.Close()
	raw := fmt.Sprintf("mailto://%s?from=sender@example.com&to=first@example.com,second@example.com&mode=insecure&format=text&retry=1&wait=0&store=no", capture.Addr())
	capture.RejectRecipientOnce("first@example.com")
	stdout, stderr, err := testutil.RunPythonScript(t, filepath.Join(testutil.RepoRoot(t), "internal/testutil/scripts/capture_smtp.py"), "--url", raw, "--body", "body", "--title", "title")
	if err != nil {
		t.Fatalf("Python SMTP %v %s", err, stderr)
	}
	var result struct{ Success bool }
	if json.Unmarshal([]byte(stdout), &result) != nil || !result.Success {
		t.Fatalf("Python result %s", stdout)
	}
	python := capture.Messages()
	capture.Reset()
	capture.RejectRecipientOnce("first@example.com")
	if err = notify.SendTargetURL(raw, "body", "title", "", notify.NotifyInfo); err != nil {
		t.Fatal(err)
	}
	got := capture.Messages()
	if len(python) != 2 || len(got) != 2 {
		t.Fatalf("delivered counts Python=%d Go=%d", len(python), len(got))
	}
	for i := range python {
		if !equalNormalizedSMTP(normalizeSMTPMessage(t, python[i]), normalizeSMTPMessage(t, got[i])) {
			t.Fatalf("SMTP retry delivery %d differs", i)
		}
	}
}
