package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const clicksendURL = "https://rest.clicksend.com/v3/sms/send"
const clicksendBatchSize = 1000

type ClickSendTarget struct {
	user     string
	password string
	targets  []string
	batch    bool
}

func NewClickSendTarget(target *ParsedURL) (*ClickSendTarget, error) {
	user := strings.TrimSpace(target.User)
	password := target.Password
	if rawKey, ok := target.Query["key"]; ok && rawKey != "" {
		password = rawKey
	}
	if user == "" || password == "" {
		return nil, fmt.Errorf("missing credentials")
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

	appendTarget(target.Host)
	for _, entry := range splitPath(target.Path) {
		appendTarget(entry)
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	batch := parseBool(target.Query["batch"], false)

	return &ClickSendTarget{
		user:     user,
		password: password,
		targets:  targets,
		batch:    batch,
	}, nil
}

func (c *ClickSendTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(c.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if c.batch {
		batchSize = clicksendBatchSize
	}
	spec, err := c.buildRequest(c.targets[:min(len(c.targets), batchSize)], message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (c *ClickSendTarget) Send(body, title string, notifyType NotifyType) error {
	if len(c.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if c.batch {
		batchSize = clicksendBatchSize
	}

	for index := 0; index < len(c.targets); index += batchSize {
		end := index + batchSize
		if end > len(c.targets) {
			end = len(c.targets)
		}
		spec, err := c.buildRequest(c.targets[index:end], message)
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

func (c *ClickSendTarget) buildRequest(targets []string, message string) (RequestSpec, error) {
	messages := make([]map[string]string, 0, len(targets))
	for _, target := range targets {
		messages = append(messages, map[string]string{
			"source": "php",
			"body":   message,
			"to":     "+" + target,
		})
	}

	payload := map[string]any{
		"messages": messages,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    clicksendURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "*/*",
			"Content-Type":  "application/json; charset=utf-8",
			"Authorization": basicAuthHeader(c.user, c.password),
		},
		Body: string(data),
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
