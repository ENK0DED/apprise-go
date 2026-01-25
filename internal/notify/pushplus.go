package notify

import (
	"encoding/json"
	"fmt"
)

const pushplusURL = "https://www.pushplus.plus/send"

type PushplusTarget struct {
	token string
}

func NewPushplusTarget(target *ParsedURL) (*PushplusTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	return &PushplusTarget{token: token}, nil
}

func (p *PushplusTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	resolvedTitle := title
	if resolvedTitle == "" {
		resolvedTitle = body
	}

	payload := map[string]any{
		"token":   p.token,
		"title":   resolvedTitle,
		"content": body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     pushplusURL,
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (p *PushplusTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
