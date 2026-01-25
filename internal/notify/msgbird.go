package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const messageBirdURL = "https://rest.messagebird.com/messages"

type MessageBirdTarget struct {
	apiKey  string
	source  string
	targets []string
}

func NewMessageBirdTarget(target *ParsedURL) (*MessageBirdTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	entries := splitPath(target.Path)
	sourceRaw := ""
	if len(entries) > 0 {
		sourceRaw = entries[0]
		entries = entries[1:]
	}
	if rawSource, ok := target.Query["from"]; ok && rawSource != "" {
		sourceRaw = rawSource
	}
	sourceRaw = strings.TrimSpace(sourceRaw)

	source, ok := normalizePhone(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}

	targets := []string{}
	hasTargetInput := false
	appendTarget := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		hasTargetInput = true
		if normalized, ok := normalizePhone(raw); ok {
			targets = append(targets, normalized)
		}
	}

	for _, entry := range entries {
		appendTarget(entry)
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	if len(targets) == 0 && !hasTargetInput {
		targets = append(targets, source)
	}

	return &MessageBirdTarget{
		apiKey:  apiKey,
		source:  source,
		targets: targets,
	}, nil
}

func (m *MessageBirdTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(m.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	spec, err := m.buildRequest(m.targets[0], message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (m *MessageBirdTarget) Send(body, title string, notifyType NotifyType) error {
	if len(m.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range m.targets {
		spec, err := m.buildRequest(target, message)
		if err != nil {
			return err
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (m *MessageBirdTarget) buildRequest(target, message string) (RequestSpec, error) {
	payload := url.Values{}
	payload.Set("originator", "+"+m.source)
	payload.Set("recipients", "+"+target)
	payload.Set("body", message)

	return RequestSpec{
		Method: "POST",
		URL:    messageBirdURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "*/*",
			"Content-Type":  "application/x-www-form-urlencoded",
			"Authorization": fmt.Sprintf("AccessKey %s", m.apiKey),
		},
		Body: payload.Encode(),
	}, nil
}
