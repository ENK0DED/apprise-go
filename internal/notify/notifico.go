package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	notificoURL             = "https://n.tkte.ch/h/%s/%s"
	notificoAppID           = "Apprise"
	notificoColorTeal       = "\x0310"
	notificoColorOrange     = "\x0307"
	notificoColorRed        = "\x0304"
	notificoColorLightGreen = "\x0309"
	notificoColorReset      = "\x03"
	notificoFormatBold      = "\x02"
	notificoFormatReset     = "\x0f"
)

type NotificoTarget struct {
	projectID string
	msgHook   string
	color     bool
	prefix    bool
}

func NewNotificoTarget(target *ParsedURL) (*NotificoTarget, error) {
	projectID := target.Host
	segments := splitPath(target.Path)
	if projectID == "" || len(segments) == 0 {
		return nil, fmt.Errorf("missing project or hook")
	}

	color := parseBoolWithDefault(target.Query["color"], true)
	prefix := parseBoolWithDefault(target.Query["prefix"], true)

	return &NotificoTarget{
		projectID: projectID,
		msgHook:   segments[0],
		color:     color,
		prefix:    prefix,
	}, nil
}

func (n *NotificoTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := n.formatPayload(body, notifyType)

	values := url.Values{}
	values.Set("payload", payload)

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
	}

	return RequestSpec{
		Method:  "GET",
		URL:     fmt.Sprintf(notificoURL, n.projectID, n.msgHook) + "?" + values.Encode(),
		Headers: headers,
		Body:    "",
	}, nil
}

func (n *NotificoTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := n.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (n *NotificoTarget) formatPayload(body string, notifyType NotifyType) string {
	color := ""
	token := "i"

	switch notifyType {
	case NotifyInfo:
		color = notificoColorTeal
		token = "i"
	case NotifySuccess:
		color = notificoColorLightGreen
		token = "✔"
	case NotifyWarning:
		color = notificoColorOrange
		token = "!"
	case NotifyFailure:
		color = notificoColorRed
		token = "✗"
	}

	if !n.color {
		color = ""
	}

	if !n.prefix {
		return body
	}

	var b strings.Builder
	if n.color {
		b.WriteString(color)
	}
	b.WriteString("[")
	b.WriteString(token)
	b.WriteString("]")
	if n.color {
		b.WriteString(notificoColorReset)
	}
	b.WriteString(" ")
	if n.color {
		b.WriteString(notificoFormatBold)
	}
	b.WriteString(notificoAppID)
	if n.color {
		b.WriteString(notificoFormatReset)
	}
	b.WriteString(": ")
	b.WriteString(body)
	if n.color {
		b.WriteString(notificoFormatReset)
	}
	return b.String()
}
