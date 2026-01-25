package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const pushSaferDefaultIcon = 25

type PushSaferTarget struct {
	privateKey string
	targets    []string
	sound      *int
	vibration  *int
	secure     bool
}

func NewPushSaferTarget(target *ParsedURL) (*PushSaferTarget, error) {
	privateKey := strings.TrimSpace(target.Host)
	if privateKey == "" {
		return nil, fmt.Errorf("missing private key")
	}

	targets := splitPath(target.Path)
	if toRaw := strings.TrimSpace(target.Query["to"]); toRaw != "" {
		targets = append(targets, parseDelimitedList(toRaw)...)
	}
	if len(targets) == 0 {
		targets = []string{"a"}
	}

	sound := parseOptionalInt(target.Query["sound"])
	vibration := parseOptionalInt(target.Query["vibration"])

	return &PushSaferTarget{
		privateKey: privateKey,
		targets:    targets,
		sound:      sound,
		vibration:  vibration,
		secure:     strings.EqualFold(target.Scheme, "psafers"),
	}, nil
}

func (p *PushSaferTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(p.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	spec := p.buildSpec(body, title, notifyType, p.targets[0])
	return spec, nil
}

func (p *PushSaferTarget) Send(body, title string, notifyType NotifyType) error {
	if len(p.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	for _, recipient := range p.targets {
		spec := p.buildSpec(body, title, notifyType, recipient)
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (p *PushSaferTarget) buildSpec(body, title string, notifyType NotifyType, recipient string) RequestSpec {
	values := url.Values{}
	values.Set("t", title)
	values.Set("m", body)
	values.Set("i", strconv.Itoa(pushSaferDefaultIcon))
	values.Set("c", appriseColor(notifyType))
	values.Set("d", recipient)
	values.Set("k", p.privateKey)

	if p.sound != nil {
		values.Set("s", strconv.Itoa(*p.sound))
	}
	if p.vibration != nil {
		values.Set("v", strconv.Itoa(*p.vibration))
	}

	scheme := "http"
	if p.secure {
		scheme = "https"
	}

	return RequestSpec{
		Method: "POST",
		URL:    scheme + "://www.pushsafer.com/api",
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: values.Encode(),
	}
}

func parseOptionalInt(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}
