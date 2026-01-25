package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const signalBatchSize = 10

var signalGroupRegex = regexp.MustCompile(`(?i)^[a-z0-9_=-]+$`)

type SignalTarget struct {
	host     string
	port     int
	secure   bool
	user     string
	password string
	source   string
	targets  []string
	batch    bool
	status   bool
}

func NewSignalTarget(target *ParsedURL) (*SignalTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	sourceRaw := strings.TrimSpace(target.Query["from"])
	if sourceRaw == "" {
		sourceRaw = strings.TrimSpace(target.Query["source"])
	}

	rawTargets := splitPath(target.Path)
	if sourceRaw == "" {
		if len(rawTargets) == 0 {
			return nil, fmt.Errorf("missing source")
		}
		sourceRaw = rawTargets[0]
		rawTargets = rawTargets[1:]
	}

	sourceDigits, ok := normalizePhone(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}
	source := "+" + sourceDigits

	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		rawTargets = append(rawTargets, parseDelimitedList(toValue)...)
	}

	targets := []string{}
	for _, entry := range rawTargets {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if normalized, ok := normalizePhone(entry); ok {
			targets = append(targets, "+"+normalized)
			continue
		}

		group := parseSignalGroup(entry)
		if group != "" {
			targets = append(targets, "group."+group)
		}
	}

	if len(targets) == 0 {
		targets = []string{source}
	}

	return &SignalTarget{
		host:     host,
		port:     target.Port,
		secure:   target.Scheme == "signals",
		user:     strings.TrimSpace(target.User),
		password: target.Password,
		source:   source,
		targets:  targets,
		batch:    parseBoolWithDefault(target.Query["batch"], false),
		status:   parseBoolWithDefault(target.Query["status"], false),
	}, nil
}

func (s *SignalTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	if s.status {
		message = strings.TrimSpace(notifyTypeASCII(notifyType) + " " + message)
	}

	recipients := s.targets
	if s.batch && len(recipients) > signalBatchSize {
		recipients = recipients[:signalBatchSize]
	} else if !s.batch {
		recipients = recipients[:1]
	}

	payload := map[string]any{
		"message":    message,
		"number":     s.source,
		"text_mode":  "normal",
		"recipients": recipients,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
	}
	if s.user != "" {
		headers["Authorization"] = basicAuthHeader(s.user, s.password)
	}

	return RequestSpec{
		Method:  "POST",
		URL:     s.buildURL(),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (s *SignalTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	if s.status {
		message = strings.TrimSpace(notifyTypeASCII(notifyType) + " " + message)
	}

	batchSize := 1
	if s.batch {
		batchSize = signalBatchSize
	}

	for index := 0; index < len(s.targets); index += batchSize {
		end := index + batchSize
		if end > len(s.targets) {
			end = len(s.targets)
		}

		payload := map[string]any{
			"message":    message,
			"number":     s.source,
			"text_mode":  "normal",
			"recipients": s.targets[index:end],
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		headers := map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		}
		if s.user != "" {
			headers["Authorization"] = basicAuthHeader(s.user, s.password)
		}

		spec := RequestSpec{
			Method:  "POST",
			URL:     s.buildURL(),
			Headers: headers,
			Body:    string(data),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (s *SignalTarget) buildURL() string {
	schema := "http"
	if s.secure {
		schema = "https"
	}

	url := schema + "://" + s.host
	if s.port != 0 {
		url += fmt.Sprintf(":%d", s.port)
	}
	return url + "/v2/send"
}

func parseSignalGroup(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "@group."):
		value = value[len("@group."):]
	case strings.HasPrefix(lower, "group."):
		value = value[len("group."):]
	case strings.HasPrefix(value, "@"):
		value = value[1:]
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !signalGroupRegex.MatchString(value) {
		return ""
	}
	return value
}
