package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const fortySixElksURL = "https://api.46elks.com/a1/sms"

type FortySixElksTarget struct {
	user     string
	password string
	source   string
	targets  []string
}

func NewFortySixElksTarget(target *ParsedURL) (*FortySixElksTarget, error) {
	user := strings.TrimSpace(target.User)
	password := target.Password
	if password == "" {
		return nil, fmt.Errorf("missing password")
	}
	if user == "" {
		return nil, fmt.Errorf("missing user")
	}

	source := ""
	if rawSource, ok := target.Query["from"]; ok && rawSource != "" {
		source = rawSource
	} else if target.Host != "" {
		source = target.Host
	}
	source = strings.TrimSpace(source)

	targets := []string{}
	appendTarget := func(raw string) {
		if normalized, ok := normalizeElksTarget(raw); ok {
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

	if len(targets) == 0 {
		if normalized, ok := normalizeElksTarget(source); ok {
			targets = append(targets, normalized)
		}
	}

	return &FortySixElksTarget{
		user:     user,
		password: password,
		source:   source,
		targets:  targets,
	}, nil
}

func (f *FortySixElksTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(f.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	spec, err := f.buildRequest(f.targets[0], message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (f *FortySixElksTarget) Send(body, title string, notifyType NotifyType) error {
	if len(f.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range f.targets {
		spec, err := f.buildRequest(target, message)
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

func (f *FortySixElksTarget) buildRequest(target, message string) (RequestSpec, error) {
	payload := url.Values{}
	payload.Set("to", target)
	payload.Set("message", message)
	if f.source != "" {
		payload.Set("from", f.source)
	}

	return RequestSpec{
		Method: "POST",
		URL:    fortySixElksURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "*/*",
			"Authorization": basicAuthHeader(f.user, f.password),
			"Content-Type":  "application/x-www-form-urlencoded",
		},
		Body: payload.Encode(),
	}, nil
}

func normalizeElksTarget(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	hasPlus := strings.HasPrefix(trimmed, "+")
	normalized, ok := normalizePhone(trimmed)
	if !ok {
		return "", false
	}
	if hasPlus {
		return "+" + normalized, true
	}
	return normalized, true
}
