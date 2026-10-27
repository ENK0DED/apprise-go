package notify

import (
	"testing"
	"time"
)

func TestRetrySettingsUpstreamBounds(t *testing.T) {
	for _, tc := range []struct {
		query string
		retry int
		wait  time.Duration
		bad   bool
	}{{"retry=1000&wait=999", 10, 20 * time.Second, false}, {"retry=999999999999999999999999999999", 10, 0, false}, {"retry=invalid&wait=invalid", 0, 0, false}, {"retry=-1", 0, 0, true}, {"wait=-1", 0, 0, true}, {"wait=NaN", 0, 0, true}, {"wait=Inf", 0, 0, true}, {"wait=0.25&retry=2", 2, 250 * time.Millisecond, false}} {
		parsed, err := ParseURL("json://example.com?" + tc.query)
		if err != nil {
			t.Fatal(err)
		}
		retry, wait, err := retrySettings(parsed)
		if (err != nil) != tc.bad || retry != tc.retry || wait != tc.wait {
			t.Fatalf("%s got %d %s %v", tc.query, retry, wait, err)
		}
	}
}
