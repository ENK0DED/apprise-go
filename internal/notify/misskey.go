package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const misskeyDefaultVisibility = "public"

var misskeyVisibilities = []string{
	"public",
	"home",
	"followers",
	"specified",
}

type MisskeyTarget struct {
	host       string
	port       int
	secure     bool
	token      string
	visibility string
}

func NewMisskeyTarget(target *ParsedURL) (*MisskeyTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	token := strings.TrimSpace(target.Query["token"])
	if token == "" {
		token = strings.TrimSpace(target.Password)
	}
	if token == "" && strings.TrimSpace(target.Password) == "" {
		token = strings.TrimSpace(target.User)
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	visibility := misskeyDefaultVisibility
	if raw := strings.TrimSpace(target.Query["visibility"]); raw != "" {
		normalized, ok := normalizeMisskeyVisibility(raw)
		if !ok {
			return nil, fmt.Errorf("invalid visibility")
		}
		visibility = normalized
	}

	return &MisskeyTarget{
		host:       host,
		port:       target.Port,
		secure:     strings.EqualFold(target.Scheme, "misskeys"),
		token:      token,
		visibility: visibility,
	}, nil
}

func (m *MisskeyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)
	payload := map[string]string{
		"i":          m.token,
		"text":       message,
		"visibility": m.visibility,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    m.baseURL() + "/api/notes/create",
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (m *MisskeyTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := m.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}
	return SendRequest(spec)
}

func (m *MisskeyTarget) baseURL() string {
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

func normalizeMisskeyVisibility(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return misskeyDefaultVisibility, true
	}

	for _, visibility := range misskeyVisibilities {
		if strings.HasPrefix(visibility, value) {
			return visibility, true
		}
	}

	return "", false
}
