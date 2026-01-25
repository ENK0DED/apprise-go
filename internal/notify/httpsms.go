package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const httpSMSURL = "https://api.httpsms.com/v1/messages/send"

type HttpSMSTarget struct {
	apiKey  string
	source  string
	targets []string
}

func NewHttpSMSTarget(target *ParsedURL) (*HttpSMSTarget, error) {
	apiKey := target.User
	if rawKey, ok := target.Query["key"]; ok && rawKey != "" {
		apiKey = rawKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	sourceRaw := ""
	targets := []string{}
	hasInvalidTarget := false

	appendTarget := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if normalized, ok := normalizePhone(raw); ok {
			targets = append(targets, normalized)
			return
		}
		hasInvalidTarget = true
	}

	if fromValue, ok := target.Query["from"]; ok && fromValue != "" {
		sourceRaw = fromValue
		appendTarget(target.Host)
		for _, entry := range splitPath(target.Path) {
			appendTarget(entry)
		}
	} else {
		sourceRaw = target.Host
		for _, entry := range splitPath(target.Path) {
			appendTarget(entry)
		}
	}

	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	source, ok := normalizePhone(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}

	if len(targets) == 0 && !hasInvalidTarget {
		targets = append(targets, source)
	}

	return &HttpSMSTarget{
		apiKey:  apiKey,
		source:  source,
		targets: targets,
	}, nil
}

func (h *HttpSMSTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(h.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	payload := map[string]string{
		"from":    "+" + h.source,
		"to":      "+" + h.targets[0],
		"content": message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    httpSMSURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
			"x-api-key":    h.apiKey,
		},
		Body: string(data),
	}, nil
}

func (h *HttpSMSTarget) Send(body, title string, notifyType NotifyType) error {
	if len(h.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range h.targets {
		payload := map[string]string{
			"from":    "+" + h.source,
			"to":      "+" + target,
			"content": message,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    httpSMSURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "*/*",
				"Content-Type": "application/json",
				"x-api-key":    h.apiKey,
			},
			Body: string(data),
		}

		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}
