package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const mastodonStatusPath = "/api/v1/statuses"
const mastodonDefaultVisibility = "default"

var mastodonUserPattern = regexp.MustCompile(`^[A-Za-z0-9_]+(@[A-Za-z0-9_.-]+)?$`)

type MastodonTarget struct {
	host           string
	port           int
	secure         bool
	token          string
	targets        []string
	visibility     string
	sensitive      bool
	spoiler        string
	language       string
	idempotencyKey string
}

func NewMastodonTarget(target *ParsedURL) (*MastodonTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	token := strings.TrimSpace(target.Query["token"])
	if token == "" && strings.TrimSpace(target.Password) == "" && strings.TrimSpace(target.User) != "" {
		token = strings.TrimSpace(target.User)
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	targets := []string{}
	for _, entry := range splitPath(target.Path) {
		if normalized, ok := normalizeMastodonTarget(entry); ok {
			targets = append(targets, normalized)
		}
	}

	visibility := strings.ToLower(strings.TrimSpace(target.Query["visibility"]))
	if visibility == "" {
		visibility = mastodonDefaultVisibility
	}

	sensitive := parseBoolValue(target.Query["sensitive"], false)

	return &MastodonTarget{
		host:           host,
		port:           target.Port,
		secure:         strings.EqualFold(target.Scheme, "mastodons") || strings.EqualFold(target.Scheme, "toots"),
		token:          token,
		targets:        targets,
		visibility:     visibility,
		sensitive:      sensitive,
		spoiler:        strings.TrimSpace(target.Query["spoiler"]),
		language:       strings.TrimSpace(target.Query["language"]),
		idempotencyKey: strings.TrimSpace(target.Query["key"]),
	}, nil
}

func (m *MastodonTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)
	status := message
	if len(m.targets) > 0 {
		status = strings.Join(m.targets, " ") + " " + message
	}

	payload := map[string]any{
		"status":    status,
		"sensitive": m.sensitive,
	}
	if m.visibility != "" && m.visibility != mastodonDefaultVisibility {
		payload["visibility"] = m.visibility
	}
	if m.spoiler != "" {
		payload["spoiler_text"] = m.spoiler
	}
	if m.language != "" {
		payload["language"] = m.language
	}
	if m.idempotencyKey != "" {
		payload["Idempotency-Key"] = m.idempotencyKey
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    m.baseURL() + mastodonStatusPath,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Authorization": "Bearer " + m.token,
			"Content-Type":  "application/json",
		},
		Body: string(data),
	}, nil
}

func (m *MastodonTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := m.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (m *MastodonTarget) baseURL() string {
	scheme := "http"
	if m.secure {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s", scheme, m.host)
	if m.port > 0 {
		base += fmt.Sprintf(":%d", m.port)
	}

	return base
}

func normalizeMastodonTarget(raw string) (string, bool) {
	entry := strings.TrimSpace(raw)
	if entry == "" {
		return "", false
	}
	entry = strings.TrimPrefix(entry, "@")
	if !mastodonUserPattern.MatchString(entry) {
		return "", false
	}
	return "@" + entry, true
}
