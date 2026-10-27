package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const parsePlatformPushSuffix = "/parse/push/"

type ParsePlatformTarget struct {
	appID     string
	masterKey string
	host      string
	port      int
	secure    bool
	fullpath  string
	devices   []string
}

func NewParsePlatformTarget(target *ParsedURL) (*ParsePlatformTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	appID := strings.TrimSpace(target.User)
	masterKey := strings.TrimSpace(target.Password)

	if rawApp := strings.TrimSpace(target.Query["app_id"]); rawApp != "" {
		appID = rawApp
	}
	if rawKey := strings.TrimSpace(target.Query["master_key"]); rawKey != "" {
		masterKey = rawKey
	}

	if appID == "" || masterKey == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	device := strings.ToLower(strings.TrimSpace(target.Query["device"]))
	if device == "" {
		device = "all"
	}

	devices, ok := parsePlatformDevices(device)
	if !ok {
		return nil, fmt.Errorf("invalid device")
	}

	fullpath := target.Path
	if strings.TrimSpace(fullpath) == "" {
		fullpath = "/"
	}

	secure := strings.EqualFold(target.Scheme, "parseps")

	return &ParsePlatformTarget{
		appID:     appID,
		masterKey: masterKey,
		host:      host,
		port:      target.Port,
		secure:    secure,
		fullpath:  fullpath,
		devices:   devices,
	}, nil
}

func (p *ParsePlatformTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"where": map[string]any{
			"deviceType": map[string]any{
				"$in": p.devices,
			},
		},
		"data": map[string]any{
			"title": title,
			"alert": body,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    p.buildURL(),
		Headers: map[string]string{
			"User-Agent":             "Apprise",
			"Content-Type":           "application/json",
			"X-Parse-Application-Id": p.appID,
			"X-Parse-Master-Key":     p.masterKey,
		},
		Body: string(data),
	}, nil
}

func (p *ParsePlatformTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (p *ParsePlatformTarget) buildURL() string {
	scheme := "http"
	if p.secure {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s", scheme, p.host)
	if p.port > 0 {
		base += fmt.Sprintf(":%d", p.port)
	}

	path := strings.TrimRight(p.fullpath, "/")
	return base + pythonQuotePath(path) + parsePlatformPushSuffix
}

func parsePlatformDevices(device string) ([]string, bool) {
	switch strings.ToLower(device) {
	case "all":
		return []string{"ios", "android"}, true
	case "ios":
		return []string{"ios"}, true
	case "android":
		return []string{"android"}, true
	default:
		return nil, false
	}
}

func init() {
	RegisterSchemaEntryOrdered(59, SchemaEntry{
		"attachment_support": false,
		"category":           "native",
		"details": map[string]any{
			"args": map[string]any{
				"app_id": map[string]any{
					"alias_of": "app_id",
				},
				"cto": map[string]any{
					"default":  4,
					"map_to":   "cto",
					"name":     "Socket Connect Timeout",
					"private":  false,
					"required": false,
					"type":     "float",
				},
				"device": map[string]any{
					"default":  "all",
					"map_to":   "device",
					"name":     "Device",
					"private":  false,
					"required": false,
					"type":     "choice:string",
					"values":   []any{"all", "ios", "android"},
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
				"master_key": map[string]any{
					"alias_of": "master_key",
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
			"templates": []any{"{schema}://{app_id}:{master_key}@{host}", "{schema}://{app_id}:{master_key}@{host}:{port}"},
			"tokens": map[string]any{
				"app_id": map[string]any{
					"map_to":   "app_id",
					"name":     "App ID",
					"private":  true,
					"required": true,
					"type":     "string",
				},
				"host": map[string]any{
					"map_to":   "host",
					"name":     "Hostname",
					"private":  false,
					"required": true,
					"type":     "string",
				},
				"master_key": map[string]any{
					"map_to":   "master_key",
					"name":     "Master Key",
					"private":  true,
					"required": true,
					"type":     "string",
				},
				"port": map[string]any{
					"map_to":   "port",
					"max":      65535,
					"min":      1,
					"name":     "Port",
					"private":  false,
					"required": false,
					"type":     "int",
				},
				"schema": map[string]any{
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"parsep", "parseps"},
				},
			},
		},
		"enabled":   true,
		"protocols": []string{"parsep"},
		"requirements": map[string]any{
			"details":              "",
			"packages_recommended": []any{},
			"packages_required":    []any{},
		},
		"secure_protocols": []string{"parseps"},
		"service_name":     "Parse Platform",
		"service_url":      " https://parseplatform.org/",
		"setup_url":        "https://appriseit.com/services/parseplatform/",
	})
}
