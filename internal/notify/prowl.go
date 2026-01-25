package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	prowlURL          = "https://api.prowlapp.com/publicapi/add"
	prowlAppID        = "Apprise"
	prowlDefaultLevel = 0
)

var prowlPriorityOrder = []struct {
	prefix string
	value  int
}{
	{"-2", -2},
	{"-1", -1},
	{"0", 0},
	{"1", 1},
	{"2", 2},
	{"l", -2},
	{"m", -1},
	{"n", 0},
	{"h", 1},
	{"e", 2},
}

type ProwlTarget struct {
	apiKey      string
	providerKey string
	priority    int
}

func NewProwlTarget(target *ParsedURL) (*ProwlTarget, error) {
	apiKey := target.Host
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key")
	}

	providerKey := ""
	segments := splitPath(target.Path)
	if len(segments) > 0 {
		providerKey = segments[0]
	}

	priority := prowlDefaultLevel
	if rawPriority, ok := target.Query["priority"]; ok && rawPriority != "" {
		priority = parseProwlPriority(rawPriority, priority)
	}

	return &ProwlTarget{
		apiKey:      apiKey,
		providerKey: providerKey,
		priority:    priority,
	}, nil
}

func (p *ProwlTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	values := url.Values{}
	values.Set("apikey", p.apiKey)
	values.Set("application", prowlAppID)
	values.Set("event", title)
	values.Set("description", body)
	values.Set("priority", fmt.Sprintf("%d", p.priority))
	if p.providerKey != "" {
		values.Set("providerkey", p.providerKey)
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-type": "application/x-www-form-urlencoded",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     prowlURL,
		Headers: headers,
		Body:    values.Encode(),
	}, nil
}

func (p *ProwlTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := p.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func parseProwlPriority(raw string, fallback int) int {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	for _, entry := range prowlPriorityOrder {
		if strings.HasPrefix(normalized, entry.prefix) {
			return entry.value
		}
	}

	return fallback
}
