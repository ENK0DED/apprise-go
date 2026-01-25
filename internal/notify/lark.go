package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const larkURL = "https://open.larksuite.com/open-apis/bot/v2/hook/%s"

var larkTokenPattern = regexp.MustCompile(`(?i)^[a-z0-9-]+$`)

type LarkTarget struct {
	token string
}

func NewLarkTarget(target *ParsedURL) (*LarkTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !larkTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	return &LarkTarget{token: token}, nil
}

func (l *LarkTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\n" + body
	}

	payload := map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": message,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    fmt.Sprintf(larkURL, l.token),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (l *LarkTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := l.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
