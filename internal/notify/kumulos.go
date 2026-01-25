package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const kumulosNotifyURL = "https://messages.kumulos.com/v2/notifications"

type KumulosTarget struct {
	apiKey    string
	serverKey string
}

func NewKumulosTarget(target *ParsedURL) (*KumulosTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	parts := splitPath(target.Path)
	if len(parts) == 0 {
		return nil, fmt.Errorf("missing server key")
	}
	serverKey := strings.TrimSpace(parts[0])
	if serverKey == "" {
		return nil, fmt.Errorf("missing server key")
	}

	return &KumulosTarget{
		apiKey:    apiKey,
		serverKey: serverKey,
	}, nil
}

func (k *KumulosTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"target": map[string]any{
			"broadcast": true,
		},
		"content": map[string]any{
			"title":   title,
			"message": body,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    kumulosNotifyURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/json",
			"Accept":        "application/json",
			"Authorization": basicAuthHeader(k.apiKey, k.serverKey),
		},
		Body: string(data),
	}, nil
}

func (k *KumulosTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := k.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
