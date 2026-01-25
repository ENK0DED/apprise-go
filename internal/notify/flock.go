package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	flockWebhookURL = "https://api.flock.com/hooks/sendMessage"
	flockAPIURL     = "https://api.flock.co/v1/chat.sendMessage"
)

var flockTokenRe = regexp.MustCompile(`(?i)^[a-z0-9-]+$`)
var flockUserRe = regexp.MustCompile(`(?i)^(?:@|u:)?([A-Z0-9_]+)$`)
var flockChannelRe = regexp.MustCompile(`(?i)^(?:#|g:)([A-Z0-9_]+)$`)

type FlockTarget struct {
	token        string
	botname      string
	includeImage bool
	targets      []string
}

func NewFlockTarget(target *ParsedURL) (*FlockTarget, error) {
	token := strings.TrimSpace(target.Host)
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !flockTokenRe.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	includeImage := parseBoolWithDefault(target.Query["image"], true)
	botname := strings.TrimSpace(target.User)

	entries := []string{}
	entries = append(entries, splitPath(target.Path)...)
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}

	normalizedEntries := normalizeFlockEntries(entries)
	targets := []string{}
	for _, entry := range normalizedEntries {
		if user := flockUserRe.FindStringSubmatch(entry); user != nil {
			targets = append(targets, "u:"+user[1])
			continue
		}
		if channel := flockChannelRe.FindStringSubmatch(entry); channel != nil {
			targets = append(targets, "g:"+channel[1])
			continue
		}
	}

	if len(entries) > 0 && len(targets) == 0 {
		return nil, fmt.Errorf("invalid targets")
	}

	return &FlockTarget{
		token:        token,
		botname:      botname,
		includeImage: includeImage,
		targets:      targets,
	}, nil
}

func (f *FlockTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(f.targets) == 0 {
		return f.buildRequest(body, title, notifyType, "")
	}
	return f.buildRequest(body, title, notifyType, f.targets[0])
}

func (f *FlockTarget) Send(body, title string, notifyType NotifyType) error {
	if len(f.targets) == 0 {
		spec, err := f.buildRequest(body, title, notifyType, "")
		if err != nil {
			return err
		}
		return SendRequest(spec)
	}

	for _, target := range f.targets {
		spec, err := f.buildRequest(body, title, notifyType, target)
		if err != nil {
			return err
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (f *FlockTarget) buildRequest(body, title string, notifyType NotifyType, target string) (RequestSpec, error) {
	payload := map[string]any{
		"token":   f.token,
		"flockml": f.buildFlockML(title, body),
		"sendAs": map[string]any{
			"name":         f.displayName(),
			"profileImage": f.profileImage(notifyType),
		},
	}

	url := flockWebhookURL + "/" + f.token
	if target != "" {
		payload["to"] = target
		url = flockAPIURL
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (f *FlockTarget) displayName() string {
	if f.botname != "" {
		return f.botname
	}
	return "Apprise"
}

func (f *FlockTarget) profileImage(notifyType NotifyType) any {
	if !f.includeImage {
		return nil
	}
	return appriseImageURL(notifyType, "72x72")
}

func (f *FlockTarget) buildFlockML(title, body string) string {
	escapedTitle := flockEscapeHTML(title)
	escapedBody := flockEscapeHTML(body)
	if escapedTitle != "" {
		escapedTitle = "<b>" + escapedTitle + "</b><br/>"
	}
	return "<flockml>" + escapedTitle + escapedBody + "</flockml>"
}

func flockEscapeHTML(value string) string {
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func normalizeFlockEntries(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	unique := map[string]struct{}{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		unique[entry] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	sorted := make([]string, 0, len(unique))
	for entry := range unique {
		sorted = append(sorted, entry)
	}
	sort.Strings(sorted)
	return sorted
}
