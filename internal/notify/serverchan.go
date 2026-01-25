package notify

import (
	"fmt"
	"net/url"
)

const serverChanURL = "https://sctapi.ftqq.com/%s.send"

type ServerChanTarget struct {
	token string
}

func NewServerChanTarget(target *ParsedURL) (*ServerChanTarget, error) {
	token := target.Host
	if token == "" {
		segments := splitPath(target.Path)
		if len(segments) > 0 {
			token = segments[0]
		}
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	return &ServerChanTarget{token: token}, nil
}

func (s *ServerChanTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	values := url.Values{}
	values.Set("title", title)
	values.Set("desp", body)

	_ = notifyType

	headers := map[string]string{
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded",
	}

	return RequestSpec{
		Method:  "POST",
		URL:     fmt.Sprintf(serverChanURL, s.token),
		Headers: headers,
		Body:    values.Encode(),
	}, nil
}

func (s *ServerChanTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
