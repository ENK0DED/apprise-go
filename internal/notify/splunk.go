package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const splunkURL = "https://alert.victorops.com/integrations/generic/20131114/alert/%s/%s"

var splunkTokenRe = regexp.MustCompile(`(?i)^[A-Z0-9_-]+$`)

var splunkActionOrder = []string{
	"map",
	"info",
	"acknowledgement",
	"warning",
	"recovery",
	"resolve",
	"critical",
}

var splunkMessageTypes = []string{
	"CRITICAL",
	"WARNING",
	"ACKNOWLEDGEMENT",
	"INFO",
	"RECOVERY",
}

type SplunkTarget struct {
	apiKey     string
	routingKey string
	entityID   string
	action     string
	mapping    map[NotifyType]string
}

func NewSplunkTarget(target *ParsedURL) (*SplunkTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if rawAPI := strings.TrimSpace(target.Query["apikey"]); rawAPI != "" {
		apiKey = rawAPI
	}
	if apiKey == "" || !splunkTokenRe.MatchString(apiKey) {
		return nil, fmt.Errorf("invalid apikey")
	}

	routingKey := strings.TrimSpace(target.User)
	if rawRoute := strings.TrimSpace(target.Query["routing_key"]); rawRoute != "" {
		routingKey = rawRoute
	} else if rawRoute := strings.TrimSpace(target.Query["route"]); rawRoute != "" {
		routingKey = rawRoute
	}
	if routingKey == "" || !splunkTokenRe.MatchString(routingKey) {
		return nil, fmt.Errorf("invalid routing key")
	}

	entityID := strings.TrimSpace(target.Query["entity_id"])
	if entityID == "" {
		entityID = strings.TrimSpace(target.Path)
		entityID = strings.Trim(entityID, " \r\n\t\v/")
	}
	if entityID == "" {
		entityID = "Apprise/" + routingKey
	}

	action := normalizeSplunkAction(target.Query["action"])
	if action == "" {
		action = "map"
	}

	mapping := map[NotifyType]string{
		NotifyInfo:    "INFO",
		NotifySuccess: "RECOVERY",
		NotifyWarning: "WARNING",
		NotifyFailure: "CRITICAL",
	}

	for key, value := range target.QueryPayload {
		notifyType, ok := normalizeSplunkNotifyType(key)
		if !ok {
			return nil, fmt.Errorf("invalid mapping key")
		}
		messageType := normalizeSplunkMessageType(value)
		if messageType == "" {
			return nil, fmt.Errorf("invalid mapping value")
		}
		mapping[notifyType] = messageType
	}

	return &SplunkTarget{
		apiKey:     apiKey,
		routingKey: routingKey,
		entityID:   entityID,
		action:     action,
		mapping:    mapping,
	}, nil
}

func (s *SplunkTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	messageType := s.resolveMessageType(notifyType)
	payload := map[string]any{
		"entity_id":           s.entityID,
		"message_type":        messageType,
		"entity_display_name": splunkEntityDisplayName(title),
		"state_message":       body,
		"monitoring_tool":     "Apprise",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	requestURL := fmt.Sprintf(splunkURL, s.apiKey, s.routingKey)
	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (s *SplunkTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (s *SplunkTarget) resolveMessageType(notifyType NotifyType) string {
	switch s.action {
	case "acknowledgement":
		return "ACKNOWLEDGEMENT"
	case "info":
		return "INFO"
	case "critical":
		return "CRITICAL"
	case "warning":
		return "WARNING"
	case "recovery", "resolve":
		return "RECOVERY"
	default:
		if messageType, ok := s.mapping[notifyType]; ok {
			return messageType
		}
		return "INFO"
	}
}

func splunkEntityDisplayName(title string) string {
	if strings.TrimSpace(title) == "" {
		return "Apprise Notifications"
	}
	return title
}

func normalizeSplunkAction(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	for _, action := range splunkActionOrder {
		if strings.HasPrefix(action, raw) {
			return action
		}
	}
	return ""
}

func normalizeSplunkMessageType(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	for _, messageType := range splunkMessageTypes {
		if strings.HasPrefix(messageType, raw) {
			return messageType
		}
	}
	return ""
}

func normalizeSplunkNotifyType(raw string) (NotifyType, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(string(NotifyInfo), raw):
		return NotifyInfo, true
	case strings.HasPrefix(string(NotifySuccess), raw):
		return NotifySuccess, true
	case strings.HasPrefix(string(NotifyWarning), raw):
		return NotifyWarning, true
	case strings.HasPrefix(string(NotifyFailure), raw):
		return NotifyFailure, true
	default:
		return NotifyInfo, false
	}
}
