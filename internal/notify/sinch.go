package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const sinchURLTemplate = "https://%s.sms.api.sinch.com/xms/v1/%s/batches"

var sinchRegions = map[string]struct{}{
	"us": {},
	"eu": {},
}

type SinchTarget struct {
	servicePlanID string
	apiToken      string
	source        string
	targets       []string
	region        string
}

func NewSinchTarget(target *ParsedURL) (*SinchTarget, error) {
	servicePlanID := strings.TrimSpace(target.User)
	apiToken := target.Password
	if raw := target.Query["spi"]; raw != "" {
		servicePlanID = raw
	}
	if raw := target.Query["token"]; raw != "" {
		apiToken = raw
	}
	if servicePlanID == "" || apiToken == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	sourceRaw := strings.TrimSpace(target.Host)
	if raw := target.Query["from"]; raw != "" {
		sourceRaw = raw
	} else if raw := target.Query["source"]; raw != "" {
		sourceRaw = raw
	}
	sourceDigits, ok := normalizePhoneWithBounds(sourceRaw, 5, 14)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}

	source := sourceDigits
	if len(sourceDigits) >= 11 && len(sourceDigits) <= 14 {
		source = "+" + sourceDigits
	} else if len(sourceDigits) != 5 && len(sourceDigits) != 6 {
		return nil, fmt.Errorf("invalid source length")
	}

	region := strings.ToLower(strings.TrimSpace(target.Query["region"]))
	if region == "" {
		region = "us"
	}
	if _, ok := sinchRegions[region]; !ok {
		return nil, fmt.Errorf("invalid region")
	}

	targets := []string{}
	appendTarget := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if normalized, ok := normalizePhone(raw); ok {
			targets = append(targets, "+"+normalized)
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
		if len(sourceDigits) >= 11 && len(sourceDigits) <= 14 {
			targets = append(targets, source)
		}
	}

	return &SinchTarget{
		servicePlanID: servicePlanID,
		apiToken:      apiToken,
		source:        source,
		targets:       targets,
		region:        region,
	}, nil
}

func (s *SinchTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	payload := map[string]any{
		"body": message,
		"from": s.source,
		"to":   []string{s.targets[0]},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	requestURL := fmt.Sprintf(sinchURLTemplate, s.region, s.servicePlanID)

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "*/*",
			"Authorization": fmt.Sprintf("Bearer %s", s.apiToken),
			"Content-Type":  "application/json",
		},
		Body: string(data),
	}, nil
}

func (s *SinchTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	requestURL := fmt.Sprintf(sinchURLTemplate, s.region, s.servicePlanID)

	for _, target := range s.targets {
		payload := map[string]any{
			"body": message,
			"from": s.source,
			"to":   []string{target},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    requestURL,
			Headers: map[string]string{
				"User-Agent":    "Apprise",
				"Accept":        "*/*",
				"Authorization": fmt.Sprintf("Bearer %s", s.apiToken),
				"Content-Type":  "application/json",
			},
			Body: string(data),
		}

		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}
