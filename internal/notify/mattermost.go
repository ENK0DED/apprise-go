package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

type MattermostTarget struct {
	host         string
	port         int
	secure       bool
	fullPath     string
	token        string
	username     string
	includeImage bool
	channels     []string
}

func NewMattermostTarget(target *ParsedURL) (*MattermostTarget, error) {
	if strings.TrimSpace(target.Host) == "" {
		return nil, fmt.Errorf("missing host")
	}

	segments := splitPath(target.Path)
	if len(segments) == 0 {
		return nil, fmt.Errorf("missing token")
	}
	token := segments[len(segments)-1]
	fullPath := ""
	if len(segments) > 1 {
		fullPath = "/" + strings.Join(segments[:len(segments)-1], "/")
	}

	channels := []string{}
	if channelValue, ok := target.Query["channels"]; ok && strings.TrimSpace(channelValue) != "" {
		channels = append(channels, parseDelimitedList(channelValue)...)
	}
	if channelValue, ok := target.Query["channel"]; ok && strings.TrimSpace(channelValue) != "" {
		channels = append(channels, parseDelimitedList(channelValue)...)
	}
	if channelValue, ok := target.Query["to"]; ok && strings.TrimSpace(channelValue) != "" {
		channels = append(channels, parseDelimitedList(channelValue)...)
	}

	return &MattermostTarget{
		host:         target.Host,
		port:         target.Port,
		secure:       target.Scheme == "mmosts",
		fullPath:     fullPath,
		token:        token,
		username:     strings.TrimSpace(target.User),
		includeImage: parseBool(target.Query["image"], true),
		channels:     channels,
	}, nil
}

func (m *MattermostTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := m.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (m *MattermostTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)

	payload := map[string]any{
		"text":     message,
		"icon_url": nil,
	}

	if m.includeImage {
		payload["icon_url"] = appriseImageURL(notifyType, "72x72")
	}

	username := m.username
	if username == "" {
		username = "Apprise"
	}
	payload["username"] = username

	if len(m.channels) > 0 {
		payload["channel"] = strings.TrimPrefix(m.channels[0], "#")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	scheme := "http"
	if m.secure {
		scheme = "https"
	}
	host := m.host
	if m.port != 0 {
		host = fmt.Sprintf("%s:%d", host, m.port)
	}

	path := strings.TrimRight(m.fullPath, "/")
	url := fmt.Sprintf("%s://%s%s/hooks/%s", scheme, host, path, m.token)

	return RequestSpec{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}
