package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const twitterTweetURL = "https://api.twitter.com/1.1/statuses/update.json"

type TwitterTarget struct {
	consumerKey    string
	consumerSecret string
	accessKey      string
	accessSecret   string
	mode           string
}

func NewTwitterTarget(target *ParsedURL) (*TwitterTarget, error) {
	consumerKey := strings.TrimSpace(target.Host)
	entries := splitPath(target.Path)
	if consumerKey == "" || len(entries) < 3 {
		return nil, fmt.Errorf("missing credentials")
	}

	consumerSecret := strings.TrimSpace(entries[0])
	accessKey := strings.TrimSpace(entries[1])
	accessSecret := strings.TrimSpace(entries[2])
	if consumerSecret == "" || accessKey == "" || accessSecret == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	mode := strings.TrimSpace(target.Query["mode"])
	if mode == "" {
		if strings.HasPrefix(target.Scheme, "tweet") {
			mode = "tweet"
		} else {
			mode = "dm"
		}
	}

	return &TwitterTarget{
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		accessKey:      accessKey,
		accessSecret:   accessSecret,
		mode:           strings.ToLower(mode),
	}, nil
}

func (t *TwitterTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if t.mode != "tweet" {
		return RequestSpec{}, fmt.Errorf("unsupported mode")
	}
	return t.tweetRequest(body)
}

func (t *TwitterTarget) Send(body, title string, notifyType NotifyType) error {
	if t.mode != "tweet" {
		return fmt.Errorf("unsupported mode")
	}
	spec, err := t.tweetRequest(body)
	if err != nil {
		return err
	}
	return SendRequest(spec)
}

func (t *TwitterTarget) tweetRequest(body string) (RequestSpec, error) {
	payload := url.Values{}
	payload.Set("status", body)

	auth, err := buildOAuth1Header(
		"POST",
		twitterTweetURL,
		payload,
		t.consumerKey,
		t.consumerSecret,
		t.accessKey,
		t.accessSecret,
	)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    twitterTweetURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/x-www-form-urlencoded",
			"Authorization": auth,
		},
		Body: payload.Encode(),
	}, nil
}
