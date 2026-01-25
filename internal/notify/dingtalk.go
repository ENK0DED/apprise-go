package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type DingTalkTarget struct {
	token   string
	targets []string
}

func NewDingTalkTarget(target *ParsedURL) (*DingTalkTarget, error) {
	token := strings.TrimSpace(target.Host)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if target.User != "" {
		return nil, fmt.Errorf("secret mode unsupported")
	}

	targets := splitPath(target.Path)
	if toValue, ok := target.Query["to"]; ok && strings.TrimSpace(toValue) != "" {
		targets = append(targets, parseDelimitedList(toValue)...)
	}

	return &DingTalkTarget{
		token:   token,
		targets: targets,
	}, nil
}

func (d *DingTalkTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := d.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (d *DingTalkTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)

	targets := d.targets
	if targets == nil {
		targets = []string{}
	}

	payload := map[string]any{
		"msgtype": "text",
		"at": map[string]any{
			"atMobiles": targets,
			"isAtAll":   false,
		},
		"text": map[string]any{
			"content": message,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	u := url.URL{
		Scheme: "https",
		Host:   "oapi.dingtalk.com",
		Path:   "/robot/send",
	}
	q := url.Values{}
	q.Set("access_token", d.token)
	u.RawQuery = q.Encode()

	return RequestSpec{
		Method: "POST",
		URL:    u.String(),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}
