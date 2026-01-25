package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RevoltTarget struct {
	botToken string
	targets  []string
	iconURL  string
	link     string
}

func NewRevoltTarget(target *ParsedURL) (*RevoltTarget, error) {
	botToken := strings.TrimSpace(target.Host)
	if botToken == "" {
		return nil, fmt.Errorf("missing bot token")
	}

	targets := splitPath(target.Path)
	if toValue, ok := target.Query["to"]; ok && strings.TrimSpace(toValue) != "" {
		targets = append(targets, parseDelimitedList(toValue)...)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	return &RevoltTarget{
		botToken: botToken,
		targets:  targets,
		iconURL:  strings.TrimSpace(target.Query["icon_url"]),
		link:     strings.TrimSpace(target.Query["url"]),
	}, nil
}

func (r *RevoltTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := r.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (r *RevoltTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(r.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	iconURL := r.iconURL
	if iconURL == "" {
		iconURL = appriseImageURL(notifyType, "256x256")
	}

	embed := map[string]any{
		"title":       nil,
		"description": body,
		"colour":      appriseColor(notifyType),
		"replies":     nil,
	}
	if strings.TrimSpace(title) != "" {
		if len(title) > 100 {
			title = title[:100]
		}
		embed["title"] = title
	}
	if iconURL != "" {
		embed["icon_url"] = iconURL
	}
	if r.link != "" {
		embed["url"] = r.link
	}

	payload := map[string]any{
		"embeds": []any{embed},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	url := fmt.Sprintf("https://api.revolt.chat/channels/%s/messages", r.targets[0])

	return RequestSpec{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json; charset=utf-8",
			"Content-Type": "application/json; charset=utf-8",
			"X-Bot-Token":  r.botToken,
		},
		Body: string(data),
	}, nil
}
