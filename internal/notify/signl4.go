package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const signl4URLTemplate = "https://connect.signl4.com/webhook/%s/"
const signl4DefaultTitle = "Apprise Notifications"
const signl4SourceSystem = "Apprise"

type Signl4Target struct {
	secret            string
	service           string
	location          string
	alertingScenario  string
	filtering         bool
	externalID        string
	status            string
}

func NewSignl4Target(target *ParsedURL) (*Signl4Target, error) {
	secret := strings.TrimSpace(target.Query["secret"])
	if secret == "" {
		secret = strings.TrimSpace(target.Host)
	}
	if secret == "" {
		return nil, fmt.Errorf("missing secret")
	}

	return &Signl4Target{
		secret:           secret,
		service:          strings.TrimSpace(target.Query["service"]),
		location:         strings.TrimSpace(target.Query["location"]),
		alertingScenario: strings.TrimSpace(target.Query["alerting_scenario"]),
		filtering:        parseBoolWithDefault(target.Query["filtering"], false),
		externalID:       strings.TrimSpace(target.Query["external_id"]),
		status:           strings.TrimSpace(target.Query["status"]),
	}, nil
}

func (s *Signl4Target) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := s.buildPayload(body, title, notifyType)
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    fmt.Sprintf(signl4URLTemplate, s.secret),
		Headers: map[string]string{
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (s *Signl4Target) Send(body, title string, notifyType NotifyType) error {
	payload := s.buildPayload(body, title, notifyType)
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	spec := RequestSpec{
		Method: "POST",
		URL:    fmt.Sprintf(signl4URLTemplate, s.secret),
		Headers: map[string]string{
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}

	return SendRequest(spec)
}

func (s *Signl4Target) buildPayload(body, title string, notifyType NotifyType) map[string]any {
	resolvedTitle := strings.TrimSpace(title)
	if resolvedTitle == "" {
		resolvedTitle = signl4DefaultTitle
	}

	payload := map[string]any{
		"title":            resolvedTitle,
		"body":             body,
		"X-S4-SourceSystem": signl4SourceSystem,
	}

	if s.service != "" {
		payload["X-S4-Service"] = s.service
	}
	if s.alertingScenario != "" {
		payload["X-S4-AlertingScenario"] = s.alertingScenario
	}
	if s.location != "" {
		payload["X-S4-Location"] = s.location
	}
	if s.filtering {
		payload["X-S4-Filtering"] = s.filtering
	}
	if s.externalID != "" {
		payload["X-S4-ExternalID"] = s.externalID
	}
	if s.status != "" {
		payload["X-S4-Status"] = s.status
	}

	_ = notifyType

	return payload
}
