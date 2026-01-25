package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	pushoverURL              = "https://api.pushover.net/1/messages.json"
	pushoverDefaultSound     = "pushover"
	pushoverDefaultPriority  = 0
	pushoverDefaultAppDesc   = "Apprise Notifications"
	pushoverSendToAllDevices = "ALL_DEVICES"
)

type PushoverTarget struct {
	userKey  string
	token    string
	targets  []string
	sound    string
	priority int
}

func NewPushoverTarget(target *ParsedURL) (*PushoverTarget, error) {
	userKey := target.User
	token := target.Host
	if userKey == "" || token == "" {
		return nil, fmt.Errorf("missing user key or token")
	}

	targets := splitPath(target.Path)
	if rawTargets, ok := target.Query["to"]; ok && rawTargets != "" {
		targets = append(targets, splitList(rawTargets)...)
	}

	if len(targets) == 0 {
		targets = []string{pushoverSendToAllDevices}
	}

	return &PushoverTarget{
		userKey:  userKey,
		token:    token,
		targets:  targets,
		sound:    pushoverDefaultSound,
		priority: pushoverDefaultPriority,
	}, nil
}

func (p *PushoverTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	resolvedTitle := title
	if resolvedTitle == "" {
		resolvedTitle = pushoverDefaultAppDesc
	}

	values := url.Values{}
	values.Set("token", p.token)
	values.Set("user", p.userKey)
	values.Set("priority", fmt.Sprintf("%d", p.priority))
	values.Set("title", resolvedTitle)
	values.Set("message", body)
	values.Set("device", strings.Join(p.targets, ","))
	values.Set("sound", p.sound)

	headers := map[string]string{
		"User-Agent":    "Apprise",
		"Accept":        "*/*",
		"Content-Type":  "application/x-www-form-urlencoded",
		"Authorization": basicAuthHeader(p.token, ""),
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     pushoverURL,
		Headers: headers,
		Body:    values.Encode(),
	}, nil
}

func (p *PushoverTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
