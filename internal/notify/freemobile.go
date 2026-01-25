package notify

import (
	"encoding/json"
	"fmt"
)

const freemobileURL = "https://smsapi.free-mobile.fr/sendmsg"

type FreeMobileTarget struct {
	user     string
	password string
}

func NewFreeMobileTarget(target *ParsedURL) (*FreeMobileTarget, error) {
	user := target.User
	password := target.Password
	if password == "" {
		password = target.Host
	}
	if user == "" || password == "" {
		return nil, fmt.Errorf("missing user or password")
	}

	return &FreeMobileTarget{user: user, password: password}, nil
}

func (f *FreeMobileTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\r\n" + body
	}

	payload := map[string]string{
		"user": f.user,
		"pass": f.password,
		"msg":  message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    freemobileURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (f *FreeMobileTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := f.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
