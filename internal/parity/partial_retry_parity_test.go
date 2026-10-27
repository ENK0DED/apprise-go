package parity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

// A retry must resend rejected deliveries while preserving each acceptance.
// Compare the complete trace, including lookups and provider acknowledgments.
func TestPartialRetryParity(t *testing.T) {
	defs := loadProviderDefinitions(t)
	for _, name := range sortedProviderNames(defs) {
		def := defs[name]
		for _, c := range def.Cases {
			if c.SendsNothing || c.KnownDivergence != "" || len(c.Attachments) > 0 || goldenRequestCount(t, def.Dir, c.Name) < 2 {
				continue
			}
			builder, ok := providerBuilders[name]
			if !ok {
				continue
			}
			t.Run(name+"/"+c.Name, func(t *testing.T) {
				sep := "?"
				if strings.Contains(c.URL, "?") {
					sep = "&"
				}
				raw := c.URL + sep + "retry=1&wait=0"
				expected, success := testutil.CapturePythonRequestsWithFailuresFormatResult(t, raw, c.Body, c.Title, c.BodyFormat, 1)
				parsed, err := notify.ParseURL(raw)
				if err != nil {
					t.Fatal(err)
				}
				if name == "matrix" {
					parsed.Query["store"] = "no"
				}
				body, err := notify.ConvertMessageFormatForTarget(parsed, c.Body, c.BodyFormat)
				if err != nil {
					t.Fatal(err)
				}
				restore := testutil.FailNextRequests(1)
				defer restore()
				var sendErr error
				got, err := testutil.CaptureGoRequestsResult(t, func() error {
					target, err := builder(parsed)
					if err != nil {
						return err
					}
					sendErr = notify.DispatchSendWithInput(target, parsed, body, c.Title, c.BodyFormat, notify.NotifyInfo, nil)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if success != nil && *success != (sendErr == nil) {
					t.Errorf("Python success=%v Go error=%v", *success, sendErr)
				}
				if name == "matrix" {
					data, _ := json.MarshalIndent(map[string]any{"python": expected, "go": got}, "", "  ")
					_ = os.WriteFile(filepath.Join(testutil.RepoRoot(t), "reports", "matrix-retry-"+c.Name+".json"), data, 0600)
				}
				assertRequestSpecSequenceMatchesExcept(t, expected, got, caseVolatileHeaders(def, c))
			})
		}
	}
}
