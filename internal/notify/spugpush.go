package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const spugpushURL = "https://push.spug.dev/send/"

var spugpushTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32,64}$`)

type SpugpushTarget struct {
	token string
}

func NewSpugpushTarget(target *ParsedURL) (*SpugpushTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !spugpushTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	return &SpugpushTarget{token: token}, nil
}

func (s *SpugpushTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	resolvedTitle := title
	if resolvedTitle == "" {
		resolvedTitle = body
	}

	payload := map[string]any{
		"title":   resolvedTitle,
		"content": body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    spugpushURL + s.token,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (s *SpugpushTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
