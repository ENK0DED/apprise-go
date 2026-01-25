package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	pushDeerDefaultHost = "api2.pushdeer.com"
	pushDeerPath        = "/message/push"
)

type PushDeerTarget struct {
	scheme  string
	host    string
	port    int
	pushKey string
}

func NewPushDeerTarget(target *ParsedURL) (*PushDeerTarget, error) {
	segments := splitPath(target.Path)
	pushKey := ""
	host := target.Host

	if len(segments) == 0 {
		pushKey = target.Host
		host = ""
	} else {
		pushKey = segments[len(segments)-1]
	}

	if pushKey == "" {
		return nil, fmt.Errorf("missing pushkey")
	}

	scheme := "http"
	if strings.ToLower(target.Scheme) == "pushdeers" {
		scheme = "https"
	}

	resolvedHost := host
	if resolvedHost == "" {
		resolvedHost = pushDeerDefaultHost
	}

	port := target.Port
	if port == 0 {
		if scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}

	return &PushDeerTarget{
		scheme:  scheme,
		host:    resolvedHost,
		port:    port,
		pushKey: pushKey,
	}, nil
}

func (p *PushDeerTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := url.Values{}
	payload.Set("text", chooseTitle(body, title))
	payload.Set("type", "text")
	if title == "" {
		payload.Set("desp", "")
	} else {
		payload.Set("desp", body)
	}

	u := url.URL{
		Scheme: p.scheme,
		Host:   fmt.Sprintf("%s:%d", p.host, p.port),
		Path:   pushDeerPath,
	}
	q := url.Values{}
	q.Set("pushkey", p.pushKey)
	u.RawQuery = q.Encode()

	headers := map[string]string{
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     u.String(),
		Headers: headers,
		Body:    payload.Encode(),
	}, nil
}

func (p *PushDeerTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func chooseTitle(body, title string) string {
	if title != "" {
		return title
	}
	return body
}
