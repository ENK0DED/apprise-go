package notify

import (
	"encoding/json"
	"fmt"
)

const techulusPushURL = "https://push.techulus.com/api/v1/notify"

type TechulusPushTarget struct {
	apiKey string
}

func NewTechulusPushTarget(target *ParsedURL) (*TechulusPushTarget, error) {
	apiKey := target.Host
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	return &TechulusPushTarget{apiKey: apiKey}, nil
}

func (t *TechulusPushTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"title": title,
		"body":  body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
		"x-api-key":    t.apiKey,
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     techulusPushURL,
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (t *TechulusPushTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := t.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
