package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const wxpusherURL = "https://wxpusher.zjiecode.com/api/send/message"

var wxpusherTokenRe = regexp.MustCompile(`(?i)^AT_[^\s]+$`)
var wxpusherTopicRe = regexp.MustCompile(`^[1-9][0-9]{0,20}$`)
var wxpusherUserRe = regexp.MustCompile(`(?i)^UID_[^\s]+$`)

const (
	wxPusherContentText = 1
	wxPusherContentHTML = 2
	wxPusherContentMD   = 3
)

type WxPusherTarget struct {
	token       string
	contentType int
	topics      []int
	users       []string
}

func NewWxPusherTarget(target *ParsedURL) (*WxPusherTarget, error) {
	token := strings.TrimSpace(target.Host)
	hostTarget := ""
	if rawToken := strings.TrimSpace(target.Query["token"]); rawToken != "" {
		token = rawToken
		hostTarget = strings.TrimSpace(target.Host)
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}
	if !wxpusherTokenRe.MatchString(token) {
		return nil, fmt.Errorf("invalid token")
	}

	contentType := wxPusherContentText
	if rawFormat := strings.ToLower(strings.TrimSpace(target.Query["format"])); rawFormat != "" {
		switch rawFormat {
		case "html":
			contentType = wxPusherContentHTML
		case "markdown", "md":
			contentType = wxPusherContentMD
		default:
			contentType = wxPusherContentText
		}
	}

	entries := []string{}
	if hostTarget != "" {
		entries = append(entries, hostTarget)
	}
	entries = append(entries, splitPath(target.Path)...)
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}

	sortedEntries := normalizeWxPusherEntries(entries)

	users := []string{}
	topics := []int{}
	for _, entry := range sortedEntries {
		if wxpusherUserRe.MatchString(entry) {
			users = append(users, entry)
			continue
		}
		if wxpusherTopicRe.MatchString(entry) {
			value, err := strconv.Atoi(entry)
			if err != nil {
				continue
			}
			topics = append(topics, value)
		}
	}

	if len(users) == 0 && len(topics) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	return &WxPusherTarget{
		token:       token,
		contentType: contentType,
		topics:      topics,
		users:       users,
	}, nil
}

func (w *WxPusherTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	spec, err := w.buildRequest(body, title)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (w *WxPusherTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := w.buildRequest(body, title)
	if err != nil {
		return err
	}

	_ = notifyType

	return SendRequest(spec)
}

func (w *WxPusherTarget) buildRequest(body, title string) (RequestSpec, error) {
	payload := map[string]any{
		"appToken":    w.token,
		"content":     body,
		"summary":     title,
		"contentType": w.contentType,
		"topicIds":    w.topics,
		"uids":        w.users,
		"url":         nil,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    wxpusherURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Body: string(data),
	}, nil
}

func normalizeWxPusherEntries(entries []string) []string {
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
