package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const lineURL = "https://api.line.me/v2/bot/message/push"

var lineTargetDelimiters = regexp.MustCompile(`[ \t\r\n,#\\/]+`)

type LineTarget struct {
	deliveryTracker
	token        string
	targets      []string
	includeImage bool
}

func NewLineTarget(target *ParsedURL) (*LineTarget, error) {
	targets := splitPathSegments(target.Path)

	token := target.Query["token"]
	if token == "" {
		token = target.Host
		if token != "" && !strings.HasSuffix(token, "=") {
			for i, entry := range targets {
				if strings.HasSuffix(entry, "=") {
					token = token + "/" + strings.Join(targets[:i+1], "/")
					targets = targets[i+1:]
					break
				}
			}
		}
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("missing token")
	}

	includeImage := parseBool(target.Query["image"], true)

	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range lineTargetDelimiters.Split(toValue, -1) {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				targets = append(targets, entry)
			}
		}
	}

	// An empty target list is not refused here. Upstream builds the object
	// and reports the failure when the send is attempted; both make no
	// request and both report failure, so matching upstream keeps the rest
	// of a configuration file behaving identically either way. The guard
	// lives on the send path instead.
	return &LineTarget{
		token:        token,
		targets:      targets,
		includeImage: includeImage,
	}, nil
}

func (l *LineTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(l.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	return l.buildRequestForTarget(l.targets[0], body, title, notifyType)
}

func (l *LineTarget) Send(body, title string, notifyType NotifyType) error {
	// Upstream keeps going after a failed target; see sendOutcome.
	var outcome sendOutcome
	if len(l.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	for deliveryIndex0, target := range l.targets {
		spec, err := l.buildRequestForTarget(target, body, title, notifyType)
		if err != nil {
			return err
		}
		outcome.record(l.sendTrackedRequestFor(spec, "Send:0", body, title, notifyType, deliveryIndex0))
	}

	return outcome.err()
}

func (l *LineTarget) buildRequestForTarget(target, body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\r\n" + body
	}

	sender := map[string]string{
		"name": "Apprise",
	}
	if l.includeImage {
		sender["iconUrl"] = appriseImageURL(notifyType, "128x128")
	}

	payload := map[string]any{
		"to": target,
		"messages": []map[string]any{
			{
				"type":   "text",
				"text":   message,
				"sender": sender,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":    "Apprise",
		"Accept":        "*/*",
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", l.token),
	}

	return RequestSpec{
		Method:  "POST",
		URL:     lineURL,
		Headers: headers,
		Body:    string(data),
	}, nil
}

func init() {
	RegisterSchemaEntryOrdered(39, SchemaEntry{
		"attachment_support": false,
		"category":           "native",
		"details": map[string]any{
			"args": map[string]any{
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
				"image": map[string]any{
					"default":  true,
					"map_to":   "include_image",
					"name":     "Include Image",
					"private":  false,
					"required": false,
					"type":     "bool",
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
			"templates": []any{"{schema}://{token}/{targets}"},
			"tokens": map[string]any{
				"schema": map[string]any{
					"default":  "line",
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"line"},
				},
				"target_user": map[string]any{
					"map_to":   "targets",
					"name":     "Target User",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"targets": map[string]any{
					"delim":    []any{"/"},
					"group":    []any{"target_user"},
					"map_to":   "targets",
					"name":     "Targets",
					"private":  false,
					"required": true,
					"type":     "list:string",
				},
				"token": map[string]any{
					"map_to":   "token",
					"name":     "Access Token",
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
		"secure_protocols": []string{"line"},
		"service_name":     "Line",
		"service_url":      "https://line.me/",
		"setup_url":        "https://appriseit.com/services/line/",
	})
}
