package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const pushbulletURL = "https://api.pushbullet.com/v2/pushes"

type PushbulletTarget struct {
	accessToken string
	target      string
}

func NewPushbulletTarget(target *ParsedURL) (*PushbulletTarget, error) {
	accessToken := target.Host
	if accessToken == "" {
		return nil, fmt.Errorf("missing access token")
	}

	targets := splitPath(target.Path)
	if rawTargets, ok := target.Query["to"]; ok && rawTargets != "" {
		targets = append(targets, splitList(rawTargets)...)
	}

	selected := ""
	for _, entry := range targets {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		selected = trimmed
		break
	}

	return &PushbulletTarget{
		accessToken: accessToken,
		target:      selected,
	}, nil
}

func (p *PushbulletTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"type":  "note",
		"title": title,
		"body":  body,
	}

	if p.target != "" {
		switch {
		case strings.HasPrefix(p.target, "#") && len(p.target) > 1:
			payload["channel_tag"] = p.target[1:]
		case looksLikeEmail(p.target):
			payload["email"] = p.target
		default:
			payload["device_iden"] = p.target
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":    "Apprise",
		"Accept":        "*/*",
		"Content-Type":  "application/json",
		"Authorization": basicAuthHeader(p.accessToken, ""),
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     pushbulletURL,
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (p *PushbulletTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func looksLikeEmail(value string) bool {
	at := strings.Index(value, "@")
	if at <= 0 || at == len(value)-1 {
		return false
	}
	return strings.Contains(value[at+1:], ".")
}
