package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type PushjetTarget struct {
	scheme    string
	host      string
	port      int
	secretKey string
	user      string
	password  string
}

func NewPushjetTarget(target *ParsedURL) (*PushjetTarget, error) {
	segments := splitPath(target.Path)
	secretKey := ""
	if len(segments) > 0 {
		secretKey = segments[0]
	}
	if rawSecret, ok := target.Query["secret"]; ok && rawSecret != "" {
		secretKey = rawSecret
	}
	if secretKey == "" {
		return nil, fmt.Errorf("missing secret key")
	}

	scheme := "http"
	if strings.ToLower(target.Scheme) == "pjets" {
		scheme = "https"
	}

	return &PushjetTarget{
		scheme:    scheme,
		host:      target.Host,
		port:      target.Port,
		secretKey: secretKey,
		user:      target.User,
		password:  target.Password,
	}, nil
}

func (p *PushjetTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	messageJSON, err := json.Marshal(body)
	if err != nil {
		return RequestSpec{}, err
	}
	titleJSON, err := json.Marshal(title)
	if err != nil {
		return RequestSpec{}, err
	}
	payload := fmt.Sprintf(
		`{"message": %s, "title": %s, "link": null, "level": null}`,
		string(messageJSON),
		string(titleJSON),
	)

	u := url.URL{
		Scheme: p.scheme,
		Host:   p.host,
		Path:   "/message/",
	}
	if p.port != 0 {
		u.Host = fmt.Sprintf("%s:%d", p.host, p.port)
	}
	q := url.Values{}
	q.Set("secret", p.secretKey)
	u.RawQuery = q.Encode()

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
	}
	if p.user != "" {
		headers["Authorization"] = basicAuthHeader(p.user, p.password)
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     u.String(),
		Headers: headers,
		Body:    payload,
	}, nil
}

func (p *PushjetTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
