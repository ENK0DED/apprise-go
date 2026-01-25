package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const wecombotURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s"

var wecombotKeyPattern = regexp.MustCompile(`(?i)^[A-Z0-9_-]+$`)

type WeComBotTarget struct {
	key string
}

func NewWeComBotTarget(target *ParsedURL) (*WeComBotTarget, error) {
	key := target.Host
	if rawKey, ok := target.Query["key"]; ok && rawKey != "" {
		key = rawKey
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("missing key")
	}
	if !wecombotKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("invalid key")
	}

	return &WeComBotTarget{key: key}, nil
}

func (w *WeComBotTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\r\n" + body
	}

	payload := map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": message,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    fmt.Sprintf(wecombotURL, w.key),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json; charset=utf-8",
		},
		Body: string(data),
	}, nil
}

func (w *WeComBotTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := w.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
