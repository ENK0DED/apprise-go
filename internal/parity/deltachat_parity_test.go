package parity

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

func TestDeltaChatSMTPParity(t *testing.T) {
	for _, tc := range []struct {
		name, scheme, body, title string
		attach                    bool
	}{
		{"plain", "deltachat", "Body\nsecond line", "Title", false},
		{"long-excerpt", "deltachats", strings.Repeat("世", 60), "", false},
		{"attachment", "deltachat", "Body", "Title", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := testutil.StartSMTPCapture(t)
			defer capture.Close()
			raw := fmt.Sprintf("%s://%s?from=sender@example.com&to=recipient@example.com&mode=insecure&format=html", tc.scheme, capture.Addr())
			args := []string{"--url", raw, "--body", tc.body, "--title", tc.title}
			var attachmentPaths []string
			if tc.attach {
				attachmentPaths = []string{testutil.WriteAttachmentFixture(t, "report.txt", "attachment body\n")}
				args = append(args, "--attach", attachmentPaths[0])
			}
			stdout, stderr, err := testutil.RunPythonScript(t, filepath.Join(testutil.RepoRoot(t), "internal/testutil/scripts/capture_smtp.py"), args...)
			if err != nil {
				t.Fatalf("Python SMTP: %v %s", err, stderr)
			}
			var result smtpSendResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil || !result.Success {
				t.Fatalf("Python result: %s", stdout)
			}
			messages := capture.Messages()
			if len(messages) != 1 {
				t.Fatalf("Python sent %d messages", len(messages))
			}
			python := messages[0]
			capture.Reset()
			attachments, err := notify.ParseAttachments(attachmentPaths)
			if err != nil {
				t.Fatal(err)
			}
			if err := notify.SendTargetURLWithAttachments(raw, tc.body, tc.title, "", notify.NotifyInfo, attachments); err != nil {
				t.Fatal(err)
			}
			messages = capture.Messages()
			if len(messages) != 1 {
				t.Fatalf("Go sent %d messages", len(messages))
			}
			got := messages[0]
			if !equalNormalizedSMTP(normalizeSMTPMessage(t, python), normalizeSMTPMessage(t, got)) {
				t.Fatalf("SMTP mismatch\nPython=%+v\nGo=%+v", normalizeSMTPMessage(t, python), normalizeSMTPMessage(t, got))
			}
			for name, message := range map[string]string{"Python": python.Data, "Go": got.Data} {
				parsed, err := mail.ReadMessage(strings.NewReader(message))
				if err != nil {
					t.Fatal(err)
				}
				if header := parsed.Header.Get("Chat-Version"); header != "1.0" {
					t.Fatalf("%s Chat-Version=%q", name, header)
				}
			}
		})
	}
}
