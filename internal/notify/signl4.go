package notify

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

const signl4URLTemplate = "https://connect.signl4.com/webhook/%s/"
const signl4DefaultTitle = "Apprise Notifications"
const signl4SourceSystem = "Apprise"

type Signl4Target struct {
	secret           string
	service          string
	location         string
	alertingScenario string
	filtering        bool
	action           string
	externalID       string
	status           string
}

func NewSignl4Target(target *ParsedURL) (*Signl4Target, error) {
	secret := strings.TrimSpace(target.Query["secret"])
	if secret == "" {
		secret = strings.TrimSpace(target.Host)
	}
	if secret == "" {
		return nil, fmt.Errorf("missing secret")
	}

	action := "trigger"
	if raw := strings.ToLower(strings.TrimSpace(target.Query["action"])); raw != "" {
		action = ""
		for _, candidate := range []string{"trigger", "map", "acknowledge", "resolve"} {
			if strings.HasPrefix(candidate, raw) {
				action = candidate
				break
			}
		}
		if action == "" {
			return nil, fmt.Errorf("invalid SIGNL4 action")
		}
	}
	externalID := strings.TrimSpace(target.Query["external_id"])
	if target.Query["external_id"] == "" {
		externalID = strings.TrimSpace(target.Query["dedup"])
	}
	return &Signl4Target{
		secret: secret, action: action,
		service:          strings.TrimSpace(target.Query["service"]),
		location:         strings.TrimSpace(target.Query["location"]),
		alertingScenario: strings.TrimSpace(target.Query["alerting_scenario"]),
		filtering:        parseBoolWithDefault(target.Query["filtering"], false),
		externalID:       externalID,
		status:           target.Query["status"],
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
		"title":             resolvedTitle,
		"body":              body,
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
	externalID := s.externalID
	if externalID == "" && s.action != "trigger" {
		identity := title
		if identity == "" {
			identity = strings.TrimSpace(strings.SplitN(body, "\n", 2)[0])
		}
		externalID = fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	}
	status := s.status
	if status == "" {
		switch {
		case s.action == "resolve" || s.action == "map" && notifyType == NotifySuccess:
			status = "resolved"
		case s.action == "acknowledge":
			status = "acknowledged"
		}
	}
	if externalID != "" {
		payload["X-S4-ExternalID"] = externalID
	}
	if status != "" {
		payload["X-S4-Status"] = status
	}

	return payload
}

func init() {
	RegisterSchemaEntryOrdered(94, SchemaEntry{
		"attachment_support": false,
		"category":           "native",
		"details": map[string]any{
			"args": map[string]any{
				"action": map[string]any{
					"default":  "trigger",
					"map_to":   "action",
					"name":     "Action",
					"private":  false,
					"required": false,
					"type":     "choice:string",
					"values":   []any{"trigger", "map", "acknowledge", "resolve"},
				},
				"alerting_scenario": map[string]any{
					"map_to":   "alerting_scenario",
					"name":     "Alerting Scenario",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"cto": map[string]any{
					"default":  4,
					"map_to":   "cto",
					"name":     "Socket Connect Timeout",
					"private":  false,
					"required": false,
					"type":     "float",
				},
				"dedup": map[string]any{
					"alias_of": "external_id",
				},
				"emojis": map[string]any{
					"default":  false,
					"map_to":   "emojis",
					"name":     "Interpret Emojis",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"external_id": map[string]any{
					"map_to":   "external_id",
					"name":     "External ID",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"filtering": map[string]any{
					"default":  false,
					"map_to":   "filtering",
					"name":     "Filtering",
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
				"location": map[string]any{
					"map_to":   "location",
					"name":     "Location",
					"private":  false,
					"required": false,
					"type":     "string",
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
				"service": map[string]any{
					"map_to":   "service",
					"name":     "Service",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"status": map[string]any{
					"map_to":   "status",
					"name":     "Status",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"store": map[string]any{
					"default":  true,
					"map_to":   "store",
					"name":     "Persistent Storage",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"tz": map[string]any{
					"default":  nil,
					"map_to":   "tz",
					"name":     "Timezone",
					"private":  false,
					"required": false,
					"type":     "string",
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
			"templates": []any{"{schema}://{secret}"},
			"tokens": map[string]any{
				"schema": map[string]any{
					"default":  "signl4",
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"signl4"},
				},
				"secret": map[string]any{
					"map_to":   "secret",
					"name":     "Secret",
					"private":  true,
					"required": true,
					"type":     "string",
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
		"secure_protocols": []string{"signl4"},
		"service_name":     "SIGNL4",
		"service_url":      "https://signl4.com/",
		"setup_url":        "https://appriseit.com/services/signl4/",
	})
}
