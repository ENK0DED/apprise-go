package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const matrixT2BotWebhookURL = "https://webhooks.t2bot.io/api/v1/matrix/hook/"

type MatrixTarget struct {
	token       string
	displayName string
}

func NewMatrixTarget(target *ParsedURL) (*MatrixTarget, error) {
	mode := strings.ToLower(strings.TrimSpace(target.Query["mode"]))
	if mode != "" && mode != "t2bot" {
		return nil, fmt.Errorf("unsupported matrix mode")
	}

	if len(splitPath(target.Path)) > 0 && mode != "t2bot" {
		return nil, fmt.Errorf("matrix rooms not supported")
	}

	token := strings.TrimSpace(target.Query["token"])
	if token == "" {
		token = strings.TrimSpace(target.Password)
	}
	if token == "" {
		token = strings.TrimSpace(target.Host)
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	displayName := strings.TrimSpace(target.User)
	if displayName == "" {
		displayName = "Apprise"
	}

	return &MatrixTarget{
		token:       token,
		displayName: displayName,
	}, nil
}

func (m *MatrixTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)
	payload := map[string]string{
		"displayName": m.displayName,
		"format":      "plain",
		"text":        message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    matrixT2BotWebhookURL + m.token,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (m *MatrixTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := m.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}
	return SendRequest(spec)
}
