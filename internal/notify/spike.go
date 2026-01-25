package notify

import (
	"encoding/json"
	"fmt"
)

const spikeURL = "https://api.spike.sh/v1/alerts/%s"

type SpikeTarget struct {
	token string
}

func NewSpikeTarget(target *ParsedURL) (*SpikeTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	return &SpikeTarget{token: token}, nil
}

func (s *SpikeTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title
	}

	payload := map[string]any{
		"message":     message,
		"description": body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     fmt.Sprintf(spikeURL, s.token),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (s *SpikeTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
