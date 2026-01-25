package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const webexURL = "https://api.ciscospark.com/v1/webhooks/incoming//"

var webexTokenPattern = regexp.MustCompile(`(?i)^[a-z0-9]{80,160}$`)

type WebexTeamsTarget struct {
	token string
}

func NewWebexTeamsTarget(target *ParsedURL) (*WebexTeamsTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !webexTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	return &WebexTeamsTarget{token: token}, nil
}

func (w *WebexTeamsTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\r\n" + body
	}

	payload := map[string]string{
		"markdown": message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    webexURL + w.token,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (w *WebexTeamsTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := w.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
