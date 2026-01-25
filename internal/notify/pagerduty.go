package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	pagerDutyRegionUS = "us"
	pagerDutyRegionEU = "eu"
)

var pagerDutyRegionURLs = map[string]string{
	pagerDutyRegionUS: "https://events.pagerduty.com/v2/enqueue",
	pagerDutyRegionEU: "https://events.eu.pagerduty.com/v2/enqueue",
}

var pagerDutySeverityMap = map[NotifyType]string{
	NotifyInfo:    "info",
	NotifySuccess: "info",
	NotifyWarning: "warning",
	NotifyFailure: "critical",
}

var pagerDutySeverities = []string{
	"info",
	"warning",
	"critical",
	"error",
}

type PagerDutyTarget struct {
	apiKey         string
	integrationKey string
	source         string
	component      string
	group          string
	classID        string
	click          string
	region         string
	severity       string
	includeImage   bool
	details        map[string]string
}

func NewPagerDutyTarget(target *ParsedURL) (*PagerDutyTarget, error) {
	apiKey := strings.TrimSpace(target.Query["apikey"])
	if apiKey == "" {
		apiKey = strings.TrimSpace(target.Host)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	integrationKey := strings.TrimSpace(target.Query["integrationkey"])
	if integrationKey == "" {
		integrationKey = strings.TrimSpace(target.User)
	}
	if integrationKey == "" {
		return nil, fmt.Errorf("missing integration key")
	}

	region := strings.ToLower(strings.TrimSpace(target.Query["region"]))
	if region == "" {
		region = pagerDutyRegionUS
	}
	if _, ok := pagerDutyRegionURLs[region]; !ok {
		return nil, fmt.Errorf("invalid region: %s", region)
	}

	severity := ""
	if rawSeverity := strings.ToLower(strings.TrimSpace(target.Query["severity"])); rawSeverity != "" {
		parsed := ""
		for _, candidate := range pagerDutySeverities {
			if strings.HasPrefix(candidate, rawSeverity) {
				parsed = candidate
				break
			}
		}
		if parsed == "" {
			return nil, fmt.Errorf("invalid severity: %s", rawSeverity)
		}
		severity = parsed
	}

	source := strings.TrimSpace(target.Query["source"])
	component := strings.TrimSpace(target.Query["component"])
	segments := splitPath(target.Path)
	if source == "" && len(segments) > 0 {
		source = strings.TrimSpace(segments[0])
	}
	if component == "" && len(segments) > 1 {
		component = strings.TrimSpace(segments[1])
	}
	if source == "" {
		source = "Apprise"
	}
	if component == "" {
		component = "Notification"
	}

	details := map[string]string{}
	for key, value := range target.QueryAdd {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		details[key] = value
	}

	return &PagerDutyTarget{
		apiKey:         apiKey,
		integrationKey: integrationKey,
		source:         source,
		component:      component,
		group:          strings.TrimSpace(target.Query["group"]),
		classID:        strings.TrimSpace(target.Query["class"]),
		click:          strings.TrimSpace(target.Query["click"]),
		region:         region,
		severity:       severity,
		includeImage:   parseBoolWithDefault(target.Query["image"], true),
		details:        details,
	}, nil
}

func (p *PagerDutyTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}
	return SendRequest(spec)
}

func (p *PagerDutyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if p.apiKey == "" || p.integrationKey == "" {
		return RequestSpec{}, fmt.Errorf("missing pagerduty credentials")
	}

	severity := p.severity
	if severity == "" {
		severity = pagerDutySeverityMap[notifyType]
	}

	summary := mergeTitleBody(title, body)
	if summary == "" {
		summary = body
	}

	payload := map[string]any{
		"routing_key": p.integrationKey,
		"payload": map[string]any{
			"summary":   summary,
			"severity":  severity,
			"source":    p.source,
			"component": p.component,
		},
		"client":       "Apprise",
		"event_action": "trigger",
	}

	if p.group != "" {
		payload["payload"].(map[string]any)["group"] = p.group
	}
	if p.classID != "" {
		payload["payload"].(map[string]any)["class"] = p.classID
	}
	if len(p.details) > 0 {
		customDetails := map[string]string{}
		for key, value := range p.details {
			customDetails[key] = value
		}
		payload["payload"].(map[string]any)["custom_details"] = customDetails
	}
	if p.click != "" {
		payload["links"] = []map[string]string{{
			"href": p.click,
		}}
	}
	if p.includeImage {
		payload["images"] = []map[string]string{{
			"src": appriseImageURL(notifyType, "128x128"),
			"alt": string(notifyType),
		}}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	url, ok := pagerDutyRegionURLs[p.region]
	if !ok {
		return RequestSpec{}, fmt.Errorf("invalid region: %s", p.region)
	}

	return RequestSpec{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
			"Authorization": fmt.Sprintf(
				"Token token=%s",
				p.apiKey,
			),
		},
		Body: string(data),
	}, nil
}
