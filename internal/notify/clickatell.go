package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const clickatellURL = "https://platform.clickatell.com/messages/http/send"

type ClickatellTarget struct {
	apiKey  string
	source  string
	targets []string
}

func NewClickatellTarget(target *ParsedURL) (*ClickatellTarget, error) {
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

	return &ClickatellTarget{
		apiKey:  apiKey,
		source:  source,
		targets: targets,
	}, nil
}

func (c *ClickatellTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(c.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	spec, err := c.buildRequest(c.targets[0], message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (c *ClickatellTarget) Send(body, title string, notifyType NotifyType) error {
	if len(c.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range c.targets {
		spec, err := c.buildRequest(target, message)
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

func (c *ClickatellTarget) buildRequest(target, message string) (RequestSpec, error) {
	params := url.Values{}
	params.Set("apiKey", c.apiKey)
	params.Set("content", message)
	params.Set("to", target)
	if c.source != "" {
		params.Set("from", c.source)
	}

	requestURL := clickatellURL
	if encoded := params.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	return RequestSpec{
		Method: "GET",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
	}, nil
}
