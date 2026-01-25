package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const pushyURL = "https://api.pushy.me/push"

type PushyTarget struct {
	apiKey string
	target string
	sound  string
	badge  *int
}

func NewPushyTarget(target *ParsedURL) (*PushyTarget, error) {
	apiKey := target.Host
	if rawKey, ok := target.Query["key"]; ok && rawKey != "" {
		apiKey = rawKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	targets := splitPath(target.Path)
	if rawTargets, ok := target.Query["to"]; ok && rawTargets != "" {
		targets = append(targets, splitList(rawTargets)...)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	selected := ""
	for _, entry := range targets {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "@") && len(trimmed) > 1 {
			selected = trimmed[1:]
			break
		}
		if strings.HasPrefix(trimmed, "#") && len(trimmed) > 1 {
			selected = trimmed[1:]
			break
		}
		if isAlnum(trimmed) {
			selected = trimmed
			break
		}
	}
	if selected == "" {
		return nil, fmt.Errorf("no valid targets")
	}

	sound := ""
	if rawSound, ok := target.Query["sound"]; ok && rawSound != "" {
		sound = rawSound
	}

	var badge *int
	if rawBadge, ok := target.Query["badge"]; ok && rawBadge != "" {
		if value, err := strconv.Atoi(strings.TrimSpace(rawBadge)); err == nil && value >= 0 {
			badge = &value
		}
	}

	return &PushyTarget{
		apiKey: apiKey,
		target: selected,
		sound:  sound,
		badge:  badge,
	}, nil
}

func (p *PushyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"to": p.target,
		"data": map[string]any{
			"message": body,
		},
		"notification": map[string]any{
			"body": body,
		},
	}

	if title != "" {
		payload["notification"].(map[string]any)["title"] = title
	}
	if p.sound != "" {
		payload["notification"].(map[string]any)["sound"] = p.sound
	}
	if p.badge != nil {
		payload["notification"].(map[string]any)["badge"] = *p.badge
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	u, err := url.Parse(pushyURL)
	if err != nil {
		return RequestSpec{}, err
	}
	q := url.Values{}
	q.Set("api_key", p.apiKey)
	u.RawQuery = q.Encode()

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Accepts":      "application/json",
		"Content-Type": "application/json",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     u.String(),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (p *PushyTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func isAlnum(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return value != ""
}
