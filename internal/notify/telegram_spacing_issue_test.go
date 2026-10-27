package notify_test

import (
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

// TestTelegramReportedSpacingParity covers the report in issue #78 through
// the actual format-conversion and send path for both supported dialects.
func TestTelegramReportedSpacingParity(t *testing.T) {
	inputs := []struct{ name, body, format string }{
		{"markdown", "### Server: host.domain\n\nDate/Time: **05:10:10 PM, Tuesday, 1 July 2026**\nUptime: **12:30:15 up 100 days, 1:20, 2 users, load average: 0.17, 0.06, 0.02**\n\n_Q:\tWhat do you call a boomerang that doesn't come back?\nA:\tA stick._\n\n### Logins and Active Users: Past 8 Hours\n```\nboomer     pts/0        192.0.2.10   Fri Jun  3 17:02:12 2026   still logged in\nlola       pts/2        192.0.2.10   Wed Jun  8 11:48:27 2026   still logged in\n```", "markdown"},
		{"html", "<h3>Server: host.domain</h3>\n\n<p>Date/Time: <strong>05:10:10 PM, Tuesday, 1 July 2026</strong>\nUptime: <strong>12:30:15 up 100 days, 1:20, 2 users, load average: 0.17, 0.06, 0.02</strong></p>\n\n<p><em>Q: What do you call a boomerang that doesn't come back?\nA:  A stick.</em></p>\n\n<h3>Logins and Active Users: Past 8 Hours</h3>\n\n<pre>\nboomer     pts/0        192.0.2.10   Fri Jun  3 17:02:12 2026   still logged in\nlola       pts/2        192.0.2.10   Wed Jun  8 11:48:27 2026   still logged in\n</pre>", "html"},
	}
	for _, input := range inputs {
		for _, dialect := range []string{"v1", "v2"} {
			t.Run(input.name+"/"+dialect, func(t *testing.T) {
				assertTelegramReportedSendParity(t, "tgram://123456:abcdef/7890/?format=markdown&mdv="+dialect, input.body, "", input.format)
			})
		}
		t.Run(input.name+"/html", func(t *testing.T) {
			assertTelegramReportedSendParity(t, "tgram://123456:abcdef/7890/?format=html", input.body, "", input.format)
		})
	}
}

func assertTelegramReportedSendParity(t *testing.T, rawURL, body, title, format string) {
	t.Helper()
	testutil.RequirePythonApprise(t)
	pythonSpecs := testutil.CapturePythonRequestsWithFormat(t, rawURL, body, title, format)
	goSpecs := testutil.CaptureGoRequests(t, func() error {
		return notify.SendTargetURL(rawURL, body, title, format, notify.NotifyInfo)
	})
	testutil.AssertRequestSpecSequenceMatches(t, pythonSpecs, goSpecs)
}
