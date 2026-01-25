package notify

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const chanifyURL = "https://api.chanify.net/v1/sender/%s/"

var chanifyTokenPattern = regexp.MustCompile(`(?i)^[A-Z0-9._-]+$`)

type ChanifyTarget struct {
	token string
}

func NewChanifyTarget(target *ParsedURL) (*ChanifyTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !chanifyTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	return &ChanifyTarget{token: token}, nil
}

func (c *ChanifyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	text := body
	if title != "" {
		text = title + "\r\n" + body
	}

	values := url.Values{}
	values.Set("text", text)

	_ = title
	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    fmt.Sprintf(chanifyURL, c.token),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: values.Encode(),
	}, nil
}

func (c *ChanifyTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := c.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
