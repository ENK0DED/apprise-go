package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

type RocketChatTarget struct {
	webhook string
	host    string
	port    int
	secure  bool
	avatar  bool
	targets []string
}

func NewRocketChatTarget(target *ParsedURL) (*RocketChatTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	mode := strings.ToLower(strings.TrimSpace(target.Query["mode"]))
	if mode != "" && mode != "webhook" {
		return nil, fmt.Errorf("unsupported mode: %s", mode)
	}

	webhook := strings.TrimSpace(target.User)
	if target.Password != "" {
		webhook = strings.TrimSpace(target.Password)
	}
	if override, ok := target.Query["webhook"]; ok && strings.TrimSpace(override) != "" {
		webhook = strings.TrimSpace(override)
	}
	if webhook == "" {
		return nil, fmt.Errorf("missing webhook")
	}

	avatar := parseBool(target.Query["avatar"], true)

	targets := splitPath(target.Path)
	if toValue, ok := target.Query["to"]; ok && strings.TrimSpace(toValue) != "" {
		targets = append(targets, parseDelimitedList(toValue)...)
	}

	return &RocketChatTarget{
		webhook: webhook,
		host:    host,
		port:    target.Port,
		secure:  target.Scheme == "rockets",
		avatar:  avatar,
		targets: targets,
	}, nil
}

func (r *RocketChatTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := r.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (r *RocketChatTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"text": mergeTitleBody(title, body),
	}
	if r.avatar {
		payload["avatar"] = appriseImageURL(notifyType, "128x128")
	}
	if len(r.targets) > 0 {
		payload["channel"] = r.targets[0]
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	scheme := "http"
	if r.secure {
		scheme = "https"
	}
	host := r.host
	if r.port != 0 {
		host = fmt.Sprintf("%s:%d", host, r.port)
	}
	url := fmt.Sprintf("%s://%s/hooks/%s", scheme, host, r.webhook)

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
	}
	return RequestSpec{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    string(data),
	}, nil
}
