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
	deliveryTracker
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
	// Upstream keeps going after a failed target; see sendOutcome.
	var outcome sendOutcome
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

		outcome.record(b.sendTrackedRequestFor(spec, "Send:0", body, title, notifyType))
	}

	_ = notifyType

	return outcome.err()
}

func (b *BurstSMSTarget) buildPayload(message string, recipients []string) url.Values {
	payload := url.Values{}
	payload.Set("countrycode", b.country)
	payload.Set("message", message)
	payload.Set("from", b.source)
	payload.Set("to", strings.Join(recipients, ","))
	return payload
}

func init() {
	RegisterSchemaEntryOrdered(123, SchemaEntry{
		"attachment_support": false,
		"category":           "native",
		"details": map[string]any{
			"args": map[string]any{
				"batch": map[string]any{
					"default":  false,
					"map_to":   "batch",
					"name":     "Batch Mode",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"country": map[string]any{
					"default":  "us",
					"map_to":   "country",
					"name":     "Country",
					"private":  false,
					"required": false,
					"type":     "choice:string",
					"values":   []any{"au", "nz", "gb", "us"},
				},
				"cto": map[string]any{
					"default":  4,
					"map_to":   "cto",
					"name":     "Socket Connect Timeout",
					"private":  false,
					"required": false,
					"type":     "float",
				},
				"emojis": map[string]any{
					"default":  false,
					"map_to":   "emojis",
					"name":     "Interpret Emojis",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"format": map[string]any{
					"default":   "text",
					"map_to":    "format",
					"name":      "Notify Format",
					"private":   false,
					"required":  false,
					"supported": []any{"text"},
					"type":      "choice:string",
					"values":    []any{"html", "markdown", "text"},
				},
				"from": map[string]any{
					"alias_of": "sender_id",
				},
				"key": map[string]any{
					"alias_of": "apikey",
				},
				"optional": map[string]any{
					"default":  false,
					"map_to":   "optional",
					"name":     "Optional Service",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"overflow": map[string]any{
					"default":  "upstream",
					"map_to":   "overflow",
					"name":     "Overflow Mode",
					"private":  false,
					"required": false,
					"type":     "choice:string",
					"values":   []any{"split", "truncate", "upstream"},
				},
				"redirect": map[string]any{
					"default":  true,
					"map_to":   "redirect",
					"name":     "Follow Redirects",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"retry": map[string]any{
					"default":  0,
					"map_to":   "retry",
					"max":      10,
					"min":      0,
					"name":     "Service Retry",
					"private":  false,
					"required": false,
					"type":     "int",
				},
				"rto": map[string]any{
					"default":  4,
					"map_to":   "rto",
					"name":     "Socket Read Timeout",
					"private":  false,
					"required": false,
					"type":     "float",
				},
				"secret": map[string]any{
					"alias_of": "secret",
				},
				"store": map[string]any{
					"default":  true,
					"map_to":   "store",
					"name":     "Persistent Storage",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"to": map[string]any{
					"alias_of": "targets",
					"delim":    []any{",", " "},
				},
				"tz": map[string]any{
					"default":  nil,
					"map_to":   "tz",
					"name":     "Timezone",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"validity": map[string]any{
					"default":  0,
					"map_to":   "validity",
					"name":     "validity",
					"private":  false,
					"required": false,
					"type":     "int",
				},
				"verify": map[string]any{
					"default":  true,
					"map_to":   "verify",
					"name":     "Verify SSL",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"wait": map[string]any{
					"default":  0,
					"map_to":   "wait",
					"max":      20,
					"min":      0,
					"name":     "Inter-Retry Wait",
					"private":  false,
					"required": false,
					"type":     "float",
				},
			},
			"kwargs":    map[string]any{},
			"templates": []any{"{schema}://{apikey}:{secret}@{sender_id}/{targets}"},
			"tokens": map[string]any{
				"apikey": map[string]any{
					"map_to":   "apikey",
					"name":     "API Key",
					"private":  true,
					"regex":    []any{"^[a-z0-9]+$", "i"},
					"required": true,
					"type":     "string",
				},
				"schema": map[string]any{
					"default":  "burstsms",
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"burstsms"},
				},
				"secret": map[string]any{
					"map_to":   "secret",
					"name":     "API Secret",
					"private":  true,
					"regex":    []any{"^[a-z0-9]+$", "i"},
					"required": true,
					"type":     "string",
				},
				"sender_id": map[string]any{
					"map_to":   "source",
					"name":     "Sender ID",
					"private":  false,
					"required": true,
					"type":     "string",
				},
				"target_phone": map[string]any{
					"map_to":   "targets",
					"name":     "Target Phone No",
					"prefix":   "+",
					"private":  false,
					"regex":    []any{"^[0-9\\s)(+-]+$", "i"},
					"required": false,
					"type":     "string",
				},
				"targets": map[string]any{
					"delim":    []any{"/"},
					"group":    []any{"target_phone"},
					"map_to":   "targets",
					"name":     "Targets",
					"private":  false,
					"required": true,
					"type":     "list:string",
				},
			},
		},
		"enabled":   true,
		"protocols": nil,
		"requirements": map[string]any{
			"details":              "",
			"packages_recommended": []any{},
			"packages_required":    []any{},
		},
		"secure_protocols": []string{"burstsms"},
		"service_name":     "Burst SMS",
		"service_url":      "https://burstsms.com/",
		"setup_url":        "https://appriseit.com/services/burstsms/",
	})
}
