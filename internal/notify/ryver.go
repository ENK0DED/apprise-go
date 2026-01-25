package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	ryverModeSlack = "slack"
	ryverModeRyver = "ryver"
)

var ryverSlackNewline = regexp.MustCompile(`\r\*\n`)

type RyverTarget struct {
	organization string
	token        string
	botname      string
	mode         string
	includeImage bool
}

func NewRyverTarget(target *ParsedURL) (*RyverTarget, error) {
	organization := strings.TrimSpace(target.Host)
	if organization == "" {
		return nil, fmt.Errorf("missing organization")
	}

	segments := splitPath(target.Path)
	if len(segments) == 0 {
		return nil, fmt.Errorf("missing token")
	}
	token := segments[0]

	mode := strings.ToLower(strings.TrimSpace(target.Query["mode"]))
	if mode == "" {
		mode = ryverModeRyver
	}
	if mode != ryverModeRyver && mode != ryverModeSlack {
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	includeImage := parseBoolWithDefault(target.Query["image"], true)

	return &RyverTarget{
		organization: organization,
		token:        token,
		botname:      strings.TrimSpace(target.User),
		mode:         mode,
		includeImage: includeImage,
	}, nil
}

func (r *RyverTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := r.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (r *RyverTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	messageTitle := title
	messageBody := body
	if r.mode == ryverModeSlack {
		messageTitle = ryverSlackFormat(messageTitle)
		messageBody = ryverSlackFormat(messageBody)
	}

	message := messageBody
	if strings.TrimSpace(messageTitle) != "" {
		message = fmt.Sprintf("**%s**\r\n%s", messageTitle, messageBody)
	}

	var displayName any
	if r.botname != "" {
		displayName = r.botname
	}

	var avatar any
	if r.includeImage {
		avatar = appriseImageURL(notifyType, "72x72")
	}

	payload := map[string]any{
		"body": message,
		"createSource": map[string]any{
			"displayName": displayName,
			"avatar":      avatar,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	url := fmt.Sprintf("https://%s.ryver.com/application/webhook/%s", r.organization, r.token)

	return RequestSpec{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func ryverSlackFormat(input string) string {
	if input == "" {
		return input
	}

	formatted := ryverSlackNewline.ReplaceAllString(input, "\\n")
	formatted = strings.ReplaceAll(formatted, "&", "&amp;")
	formatted = strings.ReplaceAll(formatted, "<", "&lt;")
	formatted = strings.ReplaceAll(formatted, ">", "&gt;")
	return formatted
}
