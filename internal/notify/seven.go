package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const sevenURL = "https://gateway.seven.io/api/sms"

type SevenTarget struct {
	apiKey  string
	source  string
	flash   bool
	label   string
	targets []string
}

func NewSevenTarget(target *ParsedURL) (*SevenTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	source := ""
	if rawSource, ok := target.Query["from"]; ok && rawSource != "" {
		source = rawSource
	} else if rawSource, ok := target.Query["source"]; ok && rawSource != "" {
		source = rawSource
	}
	source = strings.TrimSpace(source)

	flash := parseBool(target.Query["flash"], false)
	label := strings.TrimSpace(target.Query["label"])

	targets := []string{}
	appendTarget := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if normalized, ok := normalizePhone(raw); ok {
			targets = append(targets, normalized)
		}
	}

	for _, entry := range splitPath(target.Path) {
		appendTarget(entry)
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	return &SevenTarget{
		apiKey:  apiKey,
		source:  source,
		flash:   flash,
		label:   label,
		targets: targets,
	}, nil
}

func (s *SevenTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	spec, err := s.buildRequest(s.targets[0], message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (s *SevenTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range s.targets {
		spec, err := s.buildRequest(target, message)
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

func (s *SevenTarget) buildRequest(target, message string) (RequestSpec, error) {
	payload := map[string]any{
		"to":   "+" + target,
		"text": message,
	}
	if s.source != "" {
		payload["from"] = s.source
	}
	if s.flash {
		payload["flash"] = s.flash
	}
	if s.label != "" {
		payload["label"] = s.label
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    sevenURL,
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"SentWith":     "Apprise",
			"X-Api-Key":    s.apiKey,
		},
		Body: string(data),
	}, nil
}
