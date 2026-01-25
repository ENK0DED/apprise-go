package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const iftttNotifyURL = "https://maker.ifttt.com/trigger/%s/with/key/%s"

type IFTTTTarget struct {
	webhookID string
	events    []string
	addTokens map[string]string
	delTokens map[string]struct{}
}

func NewIFTTTTarget(target *ParsedURL) (*IFTTTTarget, error) {
	webhookID := target.Host
	if target.User != "" {
		webhookID = target.User
	}
	if webhookID == "" {
		return nil, fmt.Errorf("missing webhook id")
	}

	events := []string{}
	if target.User != "" && target.Host != "" {
		events = append(events, target.Host)
	}
	events = append(events, splitPath(target.Path)...)
	if rawEvents, ok := target.Query["to"]; ok && rawEvents != "" {
		events = append(events, splitList(rawEvents)...)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("missing events")
	}

	return &IFTTTTarget{
		webhookID: webhookID,
		events:    events,
		addTokens: map[string]string{},
		delTokens: map[string]struct{}{},
	}, nil
}

func (t *IFTTTTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(t.events) == 0 {
		return RequestSpec{}, fmt.Errorf("missing events")
	}

	return t.buildRequestForEvent(t.events[0], body, title, notifyType)
}

func (t *IFTTTTarget) Send(body, title string, notifyType NotifyType) error {
	for _, event := range t.events {
		spec, err := t.buildRequestForEvent(event, body, title, notifyType)
		if err != nil {
			return err
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (t *IFTTTTarget) buildRequestForEvent(event, body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"value1": title,
		"value2": body,
		"value3": string(notifyType),
	}

	for key, value := range t.addTokens {
		payload[key] = value
	}

	normalized := map[string]any{}
	for key, value := range payload {
		lowerKey := strings.ToLower(key)
		if _, dropped := t.delTokens[lowerKey]; dropped {
			continue
		}
		normalized[lowerKey] = value
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
	}

	return RequestSpec{
		Method:  "POST",
		URL:     fmt.Sprintf(iftttNotifyURL, event, t.webhookID),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func splitList(raw string) []string {
	out := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
