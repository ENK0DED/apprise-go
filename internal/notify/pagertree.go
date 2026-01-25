package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

type PagerTreeTarget struct {
	integration string
	action      string
	thirdparty  string
	urgency     string
	tags        []string
}

func NewPagerTreeTarget(target *ParsedURL) (*PagerTreeTarget, error) {
	integration := strings.TrimSpace(target.Host)
	if integration == "" {
		return nil, fmt.Errorf("missing integration id")
	}

	action := strings.ToLower(strings.TrimSpace(target.Query["action"]))
	if action == "" {
		action = "create"
	}
	switch action {
	case "create", "acknowledge", "resolve":
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}

	thirdparty := strings.TrimSpace(target.Query["thirdparty"])
	if thirdparty == "" {
		thirdparty = strings.TrimSpace(target.Query["tid"])
	}

	urgency := strings.TrimSpace(target.Query["urgency"])

	tags := []string{}
	if tagValue, ok := target.Query["tags"]; ok && strings.TrimSpace(tagValue) != "" {
		tags = append(tags, parseDelimitedList(tagValue)...)
	}

	return &PagerTreeTarget{
		integration: integration,
		action:      action,
		thirdparty:  thirdparty,
		urgency:     urgency,
		tags:        tags,
	}, nil
}

func (p *PagerTreeTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (p *PagerTreeTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if p.thirdparty == "" {
		return RequestSpec{}, fmt.Errorf("missing thirdparty id")
	}

	payload := map[string]any{
		"id":         p.thirdparty,
		"event_type": p.action,
	}

	if p.action == "create" {
		eventTitle := title
		if strings.TrimSpace(eventTitle) == "" {
			eventTitle = "Apprise Notifications"
		}

		meta := map[string]any{}
		payload["title"] = eventTitle
		payload["description"] = body
		payload["meta"] = meta
		payload["tags"] = p.tags
		if p.urgency != "" {
			payload["urgency"] = p.urgency
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	url := fmt.Sprintf("https://api.pagertree.com/integration/%s", p.integration)

	return RequestSpec{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}
