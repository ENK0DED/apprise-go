package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var gotifyPriorityOrder = []struct {
	prefix string
	value  int
}{
	{"10", 10},
	{"0", 0},
	{"1", 0},
	{"2", 0},
	{"3", 3},
	{"4", 3},
	{"5", 5},
	{"6", 5},
	{"7", 5},
	{"8", 8},
	{"9", 8},
	{"l", 0},
	{"m", 3},
	{"n", 5},
	{"h", 8},
	{"e", 10},
}

type GotifyTarget struct {
	target   *ParsedURL
	token    string
	fullpath string
	priority int
}

func NewGotifyTarget(target *ParsedURL) (*GotifyTarget, error) {
	segments := splitPath(target.Path)
	if len(segments) == 0 {
		return nil, fmt.Errorf("missing token")
	}

	token := segments[len(segments)-1]
	pathSegments := segments[:len(segments)-1]
	fullpath := "/"
	if len(pathSegments) > 0 {
		fullpath = "/" + strings.Join(pathSegments, "/") + "/"
	}

	priority := 5
	if rawPriority, ok := target.Query["priority"]; ok && rawPriority != "" {
		priority = parseGotifyPriority(rawPriority, priority)
	}

	return &GotifyTarget{
		target:   target,
		token:    token,
		fullpath: fullpath,
		priority: priority,
	}, nil
}

func (g *GotifyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	scheme := "http"
	if strings.ToLower(g.target.Scheme) == "gotifys" {
		scheme = "https"
	}

	host := g.target.Host
	if g.target.Port != 0 {
		host = fmt.Sprintf("%s:%d", host, g.target.Port)
	}

	u := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   fmt.Sprintf("%smessage", g.fullpath),
	}

	payload := map[string]any{
		"priority": g.priority,
		"title":    title,
		"message":  body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json",
		"X-Gotify-Key": g.token,
	}

	return RequestSpec{
		Method:  "POST",
		URL:     u.String(),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (g *GotifyTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := g.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func parseGotifyPriority(raw string, fallback int) int {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	for _, entry := range gotifyPriorityOrder {
		if strings.HasPrefix(normalized, entry.prefix) {
			return entry.value
		}
	}

	if value, err := strconv.Atoi(normalized); err == nil {
		return value
	}

	return fallback
}
