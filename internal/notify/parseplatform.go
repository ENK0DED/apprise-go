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
	return base + path + parsePlatformPushSuffix
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
