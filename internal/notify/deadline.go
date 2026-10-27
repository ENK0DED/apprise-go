package notify

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

// SetNotificationDeadline sets the absolute deadline for a whole notification,
// including every retry and split part. A zero value disables this limit.
func SetNotificationDeadline(parsed *ParsedURL, deadline time.Time) {
	if parsed != nil {
		parsed.notificationDeadline = deadline
		parsed.deadlineConfigured = true
	}
}
func currentNotificationDeadline() time.Time {
	currentHTTPOptionsMu.RLock()
	defer currentHTTPOptionsMu.RUnlock()
	return currentHTTPOptions.deadline
}
func notificationDialer() *net.Dialer {
	return &net.Dialer{Timeout: defaultConnectTimeout, Deadline: currentNotificationDeadline()}
}

type deadlineTransport struct {
	base     http.RoundTripper
	deadline time.Time
}

func (t deadlineTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithDeadline(request.Context(), t.deadline)
	response, err := t.base.RoundTrip(request.Clone(ctx))
	if err != nil {
		cancel()
		return response, err
	}
	response.Body = &deadlineBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

type deadlineBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *deadlineBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }
func clientWithDeadline(client *http.Client, deadline time.Time) *http.Client {
	if deadline.IsZero() {
		return client
	}
	copy := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	copy.Transport = deadlineTransport{base: transport, deadline: deadline}
	return &copy
}
