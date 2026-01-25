package notify

import (
	"fmt"
	"net/url"
)

const qqURL = "https://qmsg.zendee.cn/send/%s"

type QQTarget struct {
	token string
}

func NewQQTarget(target *ParsedURL) (*QQTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	return &QQTarget{token: token}, nil
}

func (q *QQTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\n" + body
	}

	values := url.Values{}
	values.Set("msg", message)

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     fmt.Sprintf(qqURL, q.token),
		Headers: headers,
		Body:    values.Encode(),
	}, nil
}

func (q *QQTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := q.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
