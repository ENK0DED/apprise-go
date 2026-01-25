package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const smsManagerURL = "https://http-api.smsmanager.cz/Send"
const smsManagerBatchSize = 4000

var smsManagerGateways = map[string]struct{}{
	"high":    {},
	"economy": {},
	"low":     {},
	"direct":  {},
}

type SMSManagerTarget struct {
	apiKey  string
	sender  string
	gateway string
	batch   bool
	targets []string
}

func NewSMSManagerTarget(target *ParsedURL) (*SMSManagerTarget, error) {
	apiKey := strings.TrimSpace(target.User)
	if rawKey, ok := target.Query["key"]; ok && rawKey != "" {
		apiKey = rawKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	sender := ""
	if rawSender, ok := target.Query["from"]; ok && rawSender != "" {
		sender = rawSender
	} else if rawSender, ok := target.Query["sender"]; ok && rawSender != "" {
		sender = rawSender
	}
	sender = strings.TrimSpace(sender)
	if sender != "" && len(sender) > 11 {
		sender = sender[:11]
	}

	gateway := strings.ToLower(strings.TrimSpace(target.Query["gateway"]))
	if gateway == "" {
		gateway = "high"
	}
	if _, ok := smsManagerGateways[gateway]; !ok {
		return nil, fmt.Errorf("invalid gateway")
	}

	batch := parseBool(target.Query["batch"], false)

	entries := []string{}
	if target.Host != "" {
		entries = append(entries, target.Host)
	}
	entries = append(entries, splitPath(target.Path)...)
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}

	targets := []string{}
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if normalized, ok := normalizePhoneWithBounds(trimmed, 9, 14); ok {
			if strings.HasPrefix(trimmed, "+") {
				targets = append(targets, "+"+normalized)
			} else {
				targets = append(targets, normalized)
			}
		}
	}

	return &SMSManagerTarget{
		apiKey:  apiKey,
		sender:  sender,
		gateway: gateway,
		batch:   batch,
		targets: targets,
	}, nil
}

func (s *SMSManagerTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	numbers := s.targets[:1]
	if s.batch {
		numbers = s.targets[:minInt(len(s.targets), smsManagerBatchSize)]
	}
	payload := s.buildPayload(message, numbers)

	requestURL := smsManagerURL
	if encoded := payload.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	_ = notifyType

	return RequestSpec{
		Method: "GET",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent": "Apprise",
			"Accept":     "*/*",
		},
	}, nil
}

func (s *SMSManagerTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if s.batch {
		batchSize = smsManagerBatchSize
	}

	for index := 0; index < len(s.targets); index += batchSize {
		end := index + batchSize
		if end > len(s.targets) {
			end = len(s.targets)
		}
		payload := s.buildPayload(message, s.targets[index:end])
		requestURL := smsManagerURL
		if encoded := payload.Encode(); encoded != "" {
			requestURL += "?" + encoded
		}

		spec := RequestSpec{
			Method: "GET",
			URL:    requestURL,
			Headers: map[string]string{
				"User-Agent": "Apprise",
				"Accept":     "*/*",
			},
		}

		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (s *SMSManagerTarget) buildPayload(message string, targets []string) url.Values {
	payload := url.Values{}
	payload.Set("apikey", s.apiKey)
	payload.Set("gateway", s.gateway)
	payload.Set("message", message)
	payload.Set("number", strings.Join(targets, ";"))
	if s.sender != "" {
		payload.Set("sender", s.sender)
	}
	return payload
}
