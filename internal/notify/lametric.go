package notify

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const lametricDefaultPort = 8080
const lametricDefaultUser = "dev"
const lametricDefaultPriority = "info"
const lametricDefaultIconType = "none"

var lametricIconMap = map[NotifyType]string{
	NotifyInfo:    "i620",
	NotifySuccess: "i9182",
	NotifyWarning: "i9183",
	NotifyFailure: "i9184",
}

type LametricTarget struct {
	host     string
	port     int
	secure   bool
	user     string
	apiKey   string
	priority string
	iconType string
	icon     string
	cycles   int
}

func NewLametricTarget(target *ParsedURL) (*LametricTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	user := strings.TrimSpace(target.User)
	apiKey := strings.TrimSpace(target.Password)
	if user != "" && apiKey == "" {
		apiKey = user
		user = ""
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(target.Query["apikey"])
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	mode := strings.ToLower(strings.TrimSpace(target.Query["mode"]))
	if mode == "cloud" {
		return nil, fmt.Errorf("cloud mode not supported")
	}

	priority := strings.ToLower(strings.TrimSpace(target.Query["priority"]))
	if priority == "" {
		priority = lametricDefaultPriority
	}
	if !isLametricPriority(priority) {
		priority = lametricDefaultPriority
	}

	iconType := strings.ToLower(strings.TrimSpace(target.Query["icon_type"]))
	if iconType == "" {
		iconType = lametricDefaultIconType
	}
	if !isLametricIconType(iconType) {
		iconType = lametricDefaultIconType
	}

	icon := strings.TrimSpace(target.Query["icon"])
	icon = strings.TrimPrefix(icon, "#")

	cycles := 1
	if raw := strings.TrimSpace(target.Query["cycles"]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			cycles = parsed
		}
	}

	port := target.Port
	if port == 0 {
		port = lametricDefaultPort
	}

	return &LametricTarget{
		host:     host,
		port:     port,
		secure:   strings.EqualFold(target.Scheme, "lametrics"),
		user:     user,
		apiKey:   apiKey,
		priority: priority,
		iconType: iconType,
		icon:     icon,
		cycles:   cycles,
	}, nil
}

func (l *LametricTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)

	icon := l.icon
	if icon == "" {
		if mapped, ok := lametricIconMap[notifyType]; ok {
			icon = mapped
		}
	}

	payload := map[string]any{
		"priority":  l.priority,
		"icon_type": l.iconType,
		"lifetime":  120000,
		"model": map[string]any{
			"cycles": l.cycles,
			"frames": []map[string]any{
				{
					"icon": icon,
					"text": message,
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	user := l.user
	if user == "" {
		user = lametricDefaultUser
	}

	return RequestSpec{
		Method: "POST",
		URL:    l.buildURL(),
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/json",
			"Accept":        "application/json",
			"Cache-Control": "no-cache",
			"Authorization": basicAuthHeader(user, l.apiKey),
		},
		Body: string(data),
	}, nil
}

func (l *LametricTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := l.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (l *LametricTarget) buildURL() string {
	scheme := "http"
	if l.secure {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d/api/v2/device/notifications", scheme, l.host, l.port)
}

func isLametricPriority(value string) bool {
	switch value {
	case "info", "warning", "critical":
		return true
	default:
		return false
	}
}

func isLametricIconType(value string) bool {
	switch value {
	case "info", "alert", "none":
		return true
	default:
		return false
	}
}
