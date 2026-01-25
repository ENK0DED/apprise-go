package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

type MSTeamsTarget struct {
	team         string
	tokenA       string
	tokenB       string
	tokenC       string
	tokenD       string
	version      int
	includeImage bool
}

func NewMSTeamsTarget(target *ParsedURL) (*MSTeamsTarget, error) {
	entries := splitPath(target.Path)

	team := strings.TrimSpace(target.Host)
	tokenA := ""
	if team == "" {
		return nil, fmt.Errorf("missing team")
	}
	if len(entries) > 0 {
		tokenA = entries[0]
	}
	tokenB := ""
	if len(entries) > 1 {
		tokenB = entries[1]
	}
	tokenC := ""
	if len(entries) > 2 {
		tokenC = entries[2]
	}
	tokenD := ""
	if len(entries) > 3 {
		tokenD = entries[3]
	}

	version := 1
	if team != "" {
		version = 2
	}
	if tokenD != "" {
		version = 3
	}
	if rawVersion := strings.TrimSpace(target.Query["version"]); rawVersion != "" {
		if rawVersion == "1" {
			version = 1
		} else if rawVersion == "2" {
			version = 2
		} else if rawVersion == "3" {
			version = 3
		} else {
			return nil, fmt.Errorf("invalid version: %s", rawVersion)
		}
	}

	includeImage := parseBool(target.Query["image"], true)

	return &MSTeamsTarget{
		team:         team,
		tokenA:       tokenA,
		tokenB:       tokenB,
		tokenC:       tokenC,
		tokenD:       tokenD,
		version:      version,
		includeImage: includeImage,
	}, nil
}

func (m *MSTeamsTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := m.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (m *MSTeamsTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if m.tokenA == "" || m.tokenB == "" || m.tokenC == "" {
		return RequestSpec{}, fmt.Errorf("missing tokens")
	}

	var imageURL any = nil
	if m.includeImage {
		imageURL = appriseImageURL(notifyType, "72x72")
	}

	payload := map[string]any{
		"@type":    "MessageCard",
		"@context": "https://schema.org/extensions",
		"summary":  "Apprise Notifications",
		"themeColor": appriseColor(
			notifyType,
		),
		"sections": []any{
			map[string]any{
				"activityImage": imageURL,
				"activityTitle": title,
				"text":          body,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	url := ""
	switch m.version {
	case 1:
		url = fmt.Sprintf("https://outlook.office.com/webhook/%s/IncomingWebhook/%s/%s", m.tokenA, m.tokenB, m.tokenC)
	case 2:
		url = fmt.Sprintf("https://%s.webhook.office.com/webhookb2/%s/IncomingWebhook/%s/%s", m.team, m.tokenA, m.tokenB, m.tokenC)
	case 3:
		url = fmt.Sprintf("https://%s.webhook.office.com/webhookb2/%s/IncomingWebhook/%s/%s/%s", m.team, m.tokenA, m.tokenB, m.tokenC, m.tokenD)
	default:
		return RequestSpec{}, fmt.Errorf("unsupported version: %d", m.version)
	}

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
