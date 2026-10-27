package notify

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// IncidentServiceTarget implements the related alert-event APIs. Each service
// retains its own event names, authentication, payload and accepted statuses.
type IncidentServiceTarget struct {
	service, key, source, token, action, dedup, priority, region, click, route string
	image                                                                      bool
	labels                                                                     map[string]string
	triggerID                                                                  string
	retryTracking                                                              bool
}

func (t *IncidentServiceTarget) beginNotification(enabled bool) {
	t.retryTracking = enabled
	t.triggerID = ""
}

func (t *IncidentServiceTarget) beginPiece(_ int) { t.triggerID = "" }

var incidentKeyPattern = regexp.MustCompile(`(?i)^[a-z0-9_-]+$`)
var incidentSourcePattern = regexp.MustCompile(`(?i)^[a-z0-9]+$`)

func incidentChoice(value string, choices ...string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return choices[0], nil
	}
	for _, choice := range choices {
		if strings.HasPrefix(choice, value) {
			return choice, nil
		}
	}
	return "", fmt.Errorf("invalid incident argument: %s", value)
}

func NewIncidentServiceTarget(p *ParsedURL) (*IncidentServiceTarget, error) {
	t := &IncidentServiceTarget{service: p.Scheme, key: strings.TrimSpace(p.Host), source: strings.TrimSpace(p.Host), image: parseBoolWithDefault(p.Query["image"], true), labels: p.QueryAdd}
	if value := strings.TrimSpace(p.Query["key"]); value != "" {
		t.key = value
	}
	actions := []string{"map", "trigger", "resolve"}
	if t.service == "ilert" {
		actions = []string{"map", "trigger", "acknowledge", "resolve"}
	}
	var err error
	t.action, err = incidentChoice(p.Query["action"], actions...)
	if err != nil {
		return nil, err
	}
	t.dedup = strings.TrimSpace(p.Query["dedup"])
	t.click = strings.TrimSpace(p.Query["click"])
	t.route = strings.TrimSpace(p.Query["route"])
	switch t.service {
	case "incidentio":
		if source := strings.TrimSpace(p.Query["source"]); source != "" {
			t.source = source
		}
		if tokens := splitPath(p.Path); len(tokens) > 0 {
			t.token = tokens[0]
		}
		if token := strings.TrimSpace(p.Query["token"]); token != "" {
			t.token = token
		}
		if !incidentSourcePattern.MatchString(t.source) || t.token == "" || strings.ContainsAny(t.token, " \t\r\n") {
			return nil, fmt.Errorf("invalid incident.io source or token")
		}
	case "ilert":
		if !incidentKeyPattern.MatchString(t.key) {
			return nil, fmt.Errorf("invalid ilert key")
		}
		if priority := strings.TrimSpace(p.Query["priority"]); priority != "" {
			t.priority, err = incidentChoice(priority, "high", "low")
			if err != nil {
				return nil, err
			}
		}
	case "squadcast":
		if !incidentKeyPattern.MatchString(t.key) {
			return nil, fmt.Errorf("invalid Squadcast key")
		}
		t.region = strings.ToLower(strings.TrimSpace(p.Query["region"]))
		if t.region == "" {
			t.region = "us"
		}
		if t.region != "us" && t.region != "eu" {
			return nil, fmt.Errorf("invalid Squadcast region")
		}
		t.priority = strings.ToUpper(strings.TrimSpace(p.Query["priority"]))
		if len(t.priority) == 1 {
			t.priority = "P" + t.priority
		}
		if t.priority != "" && (len(t.priority) != 2 || t.priority[0] != 'P' || t.priority[1] < '1' || t.priority[1] > '5') {
			return nil, fmt.Errorf("invalid Squadcast priority")
		}
	default:
		return nil, fmt.Errorf("unknown incident service: %s", t.service)
	}
	return t, nil
}

func incidentPlainSummary(body string) string {
	text := []rune(incidentPlainSummaryUnlimited(body))
	if len(text) > 250 {
		text = text[:250]
	}
	return string(text)
}

func incidentPlainSummaryUnlimited(body string) string {
	var out strings.Builder
	punctuation := "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) && strings.ContainsRune(punctuation, rune(body[i+1])) {
			i++
		}
		out.WriteByte(body[i])
	}
	return out.String()
}

func (t *IncidentServiceTarget) BuildRequest(body, title string, kind NotifyType) (RequestSpec, error) {
	dedup := t.dedup
	if dedup == "" && t.action == "trigger" {
		if t.triggerID == "" || !t.retryTracking {
			t.triggerID = newUUIDv4()
		}
		dedup = t.triggerID
	}
	if dedup == "" {
		key := title
		if key == "" {
			key = strings.TrimSpace(strings.SplitN(body, "\n", 2)[0])
		}
		dedup = fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	}
	summary := title
	if summary == "" {
		summary = incidentPlainSummary(body)
	}
	resolve := t.action == "resolve" || (t.action == "map" && kind == NotifySuccess)
	headers := map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json"}
	var address string
	var payload map[string]any
	switch t.service {
	case "incidentio":
		status := "firing"
		if resolve {
			status = "resolved"
		}
		payload = map[string]any{"title": summary, "description": body, "status": status, "deduplication_key": dedup}
		if t.click != "" {
			payload["source_url"] = t.click
		}
		if len(t.labels) > 0 {
			payload["metadata"] = t.labels
		}
		address = "https://api.incident.io/v2/alert_events/http/" + t.source
		headers["Authorization"] = "Bearer " + t.token
	case "ilert":
		event := "ALERT"
		if resolve {
			event = "RESOLVE"
		}
		if t.action == "acknowledge" {
			event = "ACCEPT"
		}
		payload = map[string]any{"integrationKey": t.key, "eventType": event, "summary": summary, "details": body, "alertKey": dedup}
		if t.priority != "" {
			payload["priority"] = strings.ToUpper(t.priority)
		}
		if t.route != "" {
			payload["routingKey"] = t.route
		}
		if t.click != "" {
			payload["links"] = []any{map[string]string{"href": t.click}}
		}
		if t.image {
			payload["images"] = []any{map[string]string{"src": appriseImageURL(kind, "128x128"), "alt": string(kind)}}
		}
		if len(t.labels) > 0 {
			payload["labels"] = t.labels
		}
		headers["Accept"] = "application/json"
		address = "https://api.ilert.com/api/events"
	case "squadcast":
		status := "trigger"
		if resolve {
			status = "resolve"
		}
		payload = map[string]any{"message": summary, "description": body, "status": status, "event_id": dedup}
		if t.priority != "" {
			payload["priority"] = t.priority
		}
		if len(t.labels) > 0 {
			payload["tags"] = t.labels
		}
		host := "api.squadcast.com"
		if t.region == "eu" {
			host = "api.eu.squadcast.com"
		}
		address = "https://" + host + "/v2/incidents/api/" + t.key
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}
	return RequestSpec{Method: http.MethodPost, URL: address, Headers: headers, Body: string(data)}, nil
}

func (t *IncidentServiceTarget) Send(body, title string, kind NotifyType) error {
	spec, err := t.BuildRequest(body, title, kind)
	if err != nil {
		return err
	}
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == 200 || response.StatusCode == 201 || response.StatusCode == 202 || (response.StatusCode == 204 && t.service != "incidentio") {
		return nil
	}
	return &HTTPStatusError{StatusCode: response.StatusCode}
}
