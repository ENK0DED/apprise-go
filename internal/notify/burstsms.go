package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const burstSMSURL = "https://api.transmitsms.com/send-sms.json"
const burstSMSBatchSize = 500

var burstSMSCountries = map[string]struct{}{
	"au": {},
	"nz": {},
	"gb": {},
	"us": {},
}

type BurstSMSTarget struct {
	apiKey   string
	secret   string
	source   string
	country  string
	validity int
	batch    bool
	targets  []string
}

func NewBurstSMSTarget(target *ParsedURL) (*BurstSMSTarget, error) {
	apiKey := strings.TrimSpace(target.User)
	secret := target.Password
	if apiKey == "" || secret == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	source := strings.TrimSpace(target.Host)
	if rawSource, ok := target.Query["from"]; ok && rawSource != "" {
		source = rawSource
	} else if rawSource, ok := target.Query["source"]; ok && rawSource != "" {
		source = rawSource
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("missing source")
	}

	country := strings.ToLower(strings.TrimSpace(target.Query["country"]))
	if country == "" {
		country = "us"
	}
	if _, ok := burstSMSCountries[country]; !ok {
		return nil, fmt.Errorf("invalid country")
	}

	validity := 0
	if raw := strings.TrimSpace(target.Query["validity"]); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid validity")
		}
		validity = value
	}

	batch := parseBool(target.Query["batch"], false)

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

	return &BurstSMSTarget{
		apiKey:   apiKey,
		secret:   secret,
		source:   source,
		country:  country,
		validity: validity,
		batch:    batch,
		targets:  targets,
	}, nil
}

func (b *BurstSMSTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(b.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	recipients := b.targets[:1]
	if b.batch {
		recipients = b.targets[:minInt(len(b.targets), burstSMSBatchSize)]
	}

	payload := b.buildPayload(message, recipients)
	return RequestSpec{
		Method: "POST",
		URL:    burstSMSURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "application/json",
			"Content-Type":  "application/x-www-form-urlencoded",
			"Authorization": basicAuthHeader(b.apiKey, b.secret),
		},
		Body: payload.Encode(),
	}, nil
}

func (b *BurstSMSTarget) Send(body, title string, notifyType NotifyType) error {
	if len(b.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if b.batch {
		batchSize = burstSMSBatchSize
	}

	for index := 0; index < len(b.targets); index += batchSize {
		end := index + batchSize
		if end > len(b.targets) {
			end = len(b.targets)
		}
		payload := b.buildPayload(message, b.targets[index:end])
		spec := RequestSpec{
			Method: "POST",
			URL:    burstSMSURL,
			Headers: map[string]string{
				"User-Agent":    "Apprise",
				"Accept":        "application/json",
				"Content-Type":  "application/x-www-form-urlencoded",
				"Authorization": basicAuthHeader(b.apiKey, b.secret),
			},
			Body: payload.Encode(),
		}

		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (b *BurstSMSTarget) buildPayload(message string, recipients []string) url.Values {
	payload := url.Values{}
	payload.Set("countrycode", b.country)
	payload.Set("message", message)
	payload.Set("from", b.source)
	payload.Set("to", strings.Join(recipients, ","))
	return payload
}
