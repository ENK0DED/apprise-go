package notify

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const twistAPIBase = "https://api.twist.com/api/v3/"

var twistChannelIDPattern = regexp.MustCompile(`^(?:\d+:)?\d+$`)

type TwistTarget struct {
	email      string
	password   string
	channelIDs []string
	token      string
}

func NewTwistTarget(target *ParsedURL) (*TwistTarget, error) {
	user := strings.TrimSpace(target.User)
	pass := strings.TrimSpace(target.Password)
	host := strings.TrimSpace(target.Host)
	entries := splitPath(target.Path)

	email := ""
	password := ""
	if pass == "" {
		if len(entries) == 0 {
			return nil, fmt.Errorf("missing password")
		}
		password = strings.TrimSpace(entries[0])
		entries = entries[1:]
		if user != "" && host != "" {
			email = user + "@" + host
		} else if strings.Contains(host, "@") {
			email = host
		}
	} else {
		emailUser := pass
		emailHost := host
		password = user
		if emailUser != "" && emailHost != "" {
			email = emailUser + "@" + emailHost
		} else if strings.Contains(emailUser, "@") {
			email = emailUser
		}
	}

	if !isSimpleEmail(email) {
		return nil, fmt.Errorf("invalid email")
	}
	if password == "" {
		return nil, fmt.Errorf("missing password")
	}

	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}

	channelIDs := []string{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if twistChannelIDPattern.MatchString(entry) {
			channelIDs = append(channelIDs, entry)
		}
	}
	if len(channelIDs) == 0 {
		return nil, fmt.Errorf("missing channel ids")
	}

	return &TwistTarget{
		email:      email,
		password:   password,
		channelIDs: channelIDs,
	}, nil
}

func (t *TwistTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	form := url.Values{}
	form.Set("email", t.email)
	form.Set("password", t.password)
	return RequestSpec{
		Method: "POST",
		URL:    twistAPIBase + "users/login",
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
		},
		Body: form.Encode(),
	}, nil
}

func (t *TwistTarget) Send(body, title string, notifyType NotifyType) error {
	if t.token == "" {
		if err := t.login(); err != nil {
			return err
		}
	}

	for _, channelID := range t.channelIDs {
		channelID = twistChannelOnly(channelID)

		payload := url.Values{}
		payload.Set("channel_id", channelID)
		payload.Set("title", title)
		payload.Set("content", body)

		spec := RequestSpec{
			Method: "POST",
			URL:    twistAPIBase + "threads/add",
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
				"Authorization": fmt.Sprintf(
					"Bearer %s",
					t.token,
				),
			},
			Body: payload.Encode(),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType
	return nil
}

func (t *TwistTarget) login() error {
	payload := url.Values{}
	payload.Set("email", t.email)
	payload.Set("password", t.password)

	spec := RequestSpec{
		Method: "POST",
		URL:    twistAPIBase + "users/login",
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
		},
		Body: payload.Encode(),
	}

	var response struct {
		Token string `json:"token"`
	}
	if err := doJSONRequest(spec, &response); err != nil {
		return err
	}
	if response.Token == "" {
		return fmt.Errorf("missing token")
	}
	t.token = response.Token
	return nil
}

func twistChannelOnly(value string) string {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) == 0 {
		return value
	}
	return parts[len(parts)-1]
}
