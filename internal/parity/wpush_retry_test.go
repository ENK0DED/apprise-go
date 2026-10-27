package parity

import (
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

func TestWPushPartialRetryParity(t *testing.T) {
	const url = "wpush://WPUSH0123456789abcdefghijklmnopq/alpha/beta/?retry=1"
	python := testutil.CapturePythonRequestsWithFailures(t, url, "body", "title", 1)
	parsed, err := notify.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	target, err := notify.NewTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	for notification := 0; notification < 2; notification++ {
		restore := testutil.FailNextRequests(1)
		got := testutil.CaptureGoRequests(t, func() error {
			return notify.DispatchSendWithOverflow(target, parsed, "body", "title", notify.NotifyInfo, nil)
		})
		restore()
		// Alpha fails, beta succeeds, then only alpha is retried. Reusing the
		// sender for another notification must start a fresh delivery set.
		if len(got) != 3 || len(got) != len(python) {
			t.Fatalf("notification %d: python=%d go=%d, want 3", notification, len(python), len(got))
		}
		for i := range got {
			assertRequestSpecMatches(t, python[i], got[i])
		}
	}
}
