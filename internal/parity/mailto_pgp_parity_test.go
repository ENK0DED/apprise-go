package parity

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

func TestMailtoPGPCryptographicParity(t *testing.T) {
	for _, tc := range []struct {
		name, scheme, mode string
		public, private    bool
	}{{"sign", "mailto", "sign", false, true}, {"encrypt", "mailto", "encrypt", true, false}, {"sign-and-encrypt", "mailto", "sign", true, true}, {"deltachat-encrypt", "deltachat", "encrypt", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			capture := testutil.StartSMTPCapture(t)
			defer capture.Close()
			root := testutil.RepoRoot(t)
			raw := fmt.Sprintf("%s://%s?from=sender@example.com&to=recipient@example.com&mode=insecure&format=text&pgp=%s&store=no", tc.scheme, capture.Addr(), tc.mode)
			if tc.public {
				raw += "&pgppub=" + filepath.Join(root, "internal/testutil/fixtures/pgp_recipient_public.asc")
			}
			if tc.private {
				raw += "&pgpprv=" + filepath.Join(root, "internal/testutil/fixtures/pgp_sender_private.asc")
			}
			file := testutil.WriteAttachmentFixture(t, "report.txt", "attachment bytes\n")
			_, stderr, err := testutil.RunPythonScript(t, filepath.Join(root, "internal/testutil/scripts/capture_smtp.py"), "--url", raw, "--body", "secret message", "--title", "Title", "--attach", file)
			if err != nil {
				t.Fatalf("Python SMTP %v %s", err, stderr)
			}
			messages := capture.Messages()
			if len(messages) != 1 {
				t.Fatalf("Python messages=%d", len(messages))
			}
			python := messages[0].Data
			capture.Reset()
			attachments, err := notify.ParseAttachments([]string{file})
			if err != nil {
				t.Fatal(err)
			}
			if err = notify.SendTargetURLWithAttachments(raw, "secret message", "Title", "", notify.NotifyInfo, attachments); err != nil {
				t.Fatal(err)
			}
			messages = capture.Messages()
			if len(messages) != 1 {
				t.Fatalf("Go messages=%d", len(messages))
			}
			got := messages[0].Data
			receipts := map[string]string{}
			for name, message := range map[string]string{"python": python, "go": got} {
				data, _ := json.Marshal(map[string]string{"message": message})
				stdout, stderr, err := testutil.RunPythonScript(t, filepath.Join(root, "internal/testutil/scripts/pgp_receipt.py"), string(data))
				if err != nil {
					t.Fatalf("%s crypto verification %v %s", name, err, stderr)
				}
				receipts[name] = strings.TrimSpace(stdout)
			}
			if receipts["python"] != receipts["go"] {
				t.Fatalf("decrypted MIME differs\nPython %s\nGo %s", receipts["python"], receipts["go"])
			}
		})
	}
}
