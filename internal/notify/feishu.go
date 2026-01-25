package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const feishuURL = "https://open.feishu.cn/open-apis/bot/v2/hook/%s/"

var feishuTokenPattern = regexp.MustCompile(`(?i)^[A-Z0-9_-]+$`)

type FeishuTarget struct {
	token string
}

func NewFeishuTarget(target *ParsedURL) (*FeishuTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !feishuTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	return &FeishuTarget{token: token}, nil
}

func (f *FeishuTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\r\n" + body
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
		URL:    fmt.Sprintf(feishuURL, f.token),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (f *FeishuTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := f.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
