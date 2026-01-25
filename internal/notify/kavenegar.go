package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const kavenegarURL = "http://api.kavenegar.com/v1/%s/sms/send.json"

type KavenegarTarget struct {
	apiKey  string
	source  string
	targets []string
}

func NewKavenegarTarget(target *ParsedURL) (*KavenegarTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	source := strings.TrimSpace(target.User)
	if rawSource, ok := target.Query["from"]; ok && rawSource != "" {
		source = strings.TrimSpace(rawSource)
	}
	if source != "" {
		normalized, ok := normalizePhone(source)
		if !ok {
			return nil, fmt.Errorf("invalid source")
		}
		source = normalized
	}

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

	return &KavenegarTarget{
		apiKey:  apiKey,
		source:  source,
		targets: targets,
	}, nil
}

func (k *KavenegarTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(k.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	spec, err := k.buildRequest(k.targets[0], message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (k *KavenegarTarget) Send(body, title string, notifyType NotifyType) error {
	if len(k.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range k.targets {
		spec, err := k.buildRequest(target, message)
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

func (k *KavenegarTarget) buildRequest(target, message string) (RequestSpec, error) {
	params := url.Values{}
	params.Set("receptor", target)
	params.Set("message", message)
	if k.source != "" {
		params.Set("sender", k.source)
	}

	requestURL := fmt.Sprintf(kavenegarURL, k.apiKey)
	encoded := params.Encode()
	if encoded != "" {
		requestURL += "?" + encoded
	}

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent": "Apprise",
			"Accept":     "application/json",
		},
	}, nil
}
