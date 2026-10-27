package notify

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type deliveryBuildFailureTarget struct {
	deliveryTracker
	attempt int
}

func (t *deliveryBuildFailureTarget) Send(body, title string, kind NotifyType) error {
	t.attempt++
	var outcome sendOutcome
	for index, endpoint := range []string{"first", "second"} {
		if t.attempt == 1 && index == 0 {
			outcome.record(fmt.Errorf("fixture preparation failed"))
			continue
		}
		outcome.record(t.sendTrackedRequestFor(RequestSpec{Method: "POST", URL: "https://example.com/" + endpoint, Body: body}, "fixture", index))
	}
	return outcome.err()
}
func TestPartialRetryIdentitySurvivesPreparationFailure(t *testing.T) {
	calls := map[string]int{}
	old := http.DefaultTransport
	http.DefaultTransport = reviewTransport(func(r *http.Request) (*http.Response, error) {
		calls[r.URL.Path]++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	defer func() { http.DefaultTransport = old }()
	parsed, err := ParseURL("json://example.com?retry=1")
	if err != nil {
		t.Fatal(err)
	}
	target := &deliveryBuildFailureTarget{}
	if err = DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, nil); err != nil {
		t.Fatal(err)
	}
	if calls["/first"] != 1 || calls["/second"] != 1 {
		t.Fatalf("wrong deliveries after preparation recovery: %+v", calls)
	}
}
