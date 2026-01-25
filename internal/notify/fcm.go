package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

const fcmLegacyURL = "https://fcm.googleapis.com/fcm/send"

type FCMTarget struct {
	apiKey   string
	targets  []string
	color    string
	hasColor bool
}

func NewFCMTarget(target *ParsedURL) (*FCMTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" {
		apiKey = strings.TrimSpace(target.User)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	targets := make([]string, 0, 1)
	for _, entry := range splitPath(target.Path) {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			targets = append(targets, trimmed)
		}
	}
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		targets = append(targets, parseDelimitedList(toValue)...)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	color := ""
	hasColor := false
	if raw, ok := target.Query["color"]; ok {
		color = strings.TrimSpace(raw)
		hasColor = true
	}

	return &FCMTarget{
		apiKey:   apiKey,
		targets:  targets,
		color:    color,
		hasColor: hasColor,
	}, nil
}

func (f *FCMTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(f.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	spec, err := f.buildSpec(body, title, notifyType, f.targets[0])
	if err != nil {
		return RequestSpec{}, err
	}
	return spec, nil
}

func (f *FCMTarget) Send(body, title string, notifyType NotifyType) error {
	if len(f.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	for _, recipient := range f.targets {
		spec, err := f.buildSpec(body, title, notifyType, recipient)
		if err != nil {
			return err
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (f *FCMTarget) buildSpec(body, title string, notifyType NotifyType, recipient string) (RequestSpec, error) {
	payload := map[string]any{
		"notification": map[string]any{
			"notification": map[string]string{
				"title": title,
				"body":  body,
			},
		},
	}

	if color, ok := f.resolveColor(notifyType); ok {
		payload["notification"].(map[string]any)["notification"].(map[string]string)["color"] = color
	}

	if strings.HasPrefix(recipient, "#") {
		payload["to"] = "/topics/" + recipient
	} else {
		payload["to"] = recipient
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    fcmLegacyURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/json",
			"Authorization": "key=" + f.apiKey,
		},
		Body: string(data),
	}, nil
}

func (f *FCMTarget) resolveColor(notifyType NotifyType) (string, bool) {
	if !f.hasColor {
		return "", false
	}
	if f.color == "" {
		return appriseColor(notifyType), true
	}

	normalized := strings.ToLower(strings.TrimSpace(f.color))
	switch normalized {
	case "1", "true", "yes", "on", "y":
		return appriseColor(notifyType), true
	case "0", "false", "no", "off", "n":
		return "", false
	default:
		if color, ok := normalizeHexColor(normalized); ok {
			return color, true
		}
	}

	return "", false
}

func normalizeHexColor(raw string) (string, bool) {
	value := strings.TrimPrefix(raw, "#")
	if len(value) != 3 && len(value) != 6 {
		return "", false
	}

	for _, r := range value {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return "", false
		}
	}

	if len(value) == 3 {
		value = string([]byte{
			value[0], value[0],
			value[1], value[1],
			value[2], value[2],
		})
	}

	return "#" + strings.ToLower(value), true
}
