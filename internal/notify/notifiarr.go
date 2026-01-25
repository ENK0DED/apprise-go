package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const notifiarrURL = "https://notifiarr.com/api/v1/notification/apprise"
const notifiarrAppID = "Apprise"
const notifiarrAppDesc = "Apprise Notifications"

var notifiarrChannelRe = regexp.MustCompile(`(?i)^\s*(?:#|%35)?([0-9]+)`)
var notifiarrChannelDelims = regexp.MustCompile(`[ \t\r\n,#\\/]+`)
var notifiarrMentionRe = regexp.MustCompile(`(?i)\s*(?:<?@(&?)([0-9]+)>?|@([a-z0-9]+))`)

type NotifiarrTarget struct {
	apikey       string
	includeImage bool
	event        int
	source       string
	channels     []int
}

func NewNotifiarrTarget(target *ParsedURL) (*NotifiarrTarget, error) {
	apikey := strings.TrimSpace(target.Host)
	hostTarget := ""
	if rawKey := strings.TrimSpace(target.Query["apikey"]); rawKey != "" {
		apikey = rawKey
		hostTarget = strings.TrimSpace(target.Host)
	} else if rawKey := strings.TrimSpace(target.Query["key"]); rawKey != "" {
		apikey = rawKey
		hostTarget = strings.TrimSpace(target.Host)
	}
	if apikey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	includeImage := parseBoolWithDefault(target.Query["image"], false)

	source := strings.TrimSpace(target.Query["source"])
	if source == "" {
		source = strings.TrimSpace(target.Query["from"])
	}

	event := 0
	if rawEvent := strings.TrimSpace(target.Query["event"]); rawEvent != "" {
		value, err := strconv.Atoi(rawEvent)
		if err != nil {
			return nil, fmt.Errorf("invalid event")
		}
		event = value
	}

	entries := splitPath(target.Path)
	if hostTarget != "" {
		entries = append(entries, hostTarget)
	}
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, splitNotifiarrList(toValue)...)
	}

	channelValues := []string{}
	for _, entry := range entries {
		channel, ok := parseNotifiarrChannel(entry)
		if !ok {
			continue
		}
		channelValues = append(channelValues, channel)
	}

	channels := normalizeNotifiarrChannels(channelValues)

	return &NotifiarrTarget{
		apikey:       apikey,
		includeImage: includeImage,
		event:        event,
		source:       source,
		channels:     channels,
	}, nil
}

func (n *NotifiarrTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(n.channels) == 0 {
		return RequestSpec{}, fmt.Errorf("missing channels")
	}

	payload := n.buildPayload(body, title, notifyType, n.channels[0])
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    notifiarrURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
			"Accept":       "text/plain",
			"X-api-Key":    n.apikey,
		},
		Body: string(data),
	}, nil
}

func (n *NotifiarrTarget) Send(body, title string, notifyType NotifyType) error {
	if len(n.channels) == 0 {
		return fmt.Errorf("missing channels")
	}

	for _, channel := range n.channels {
		payload := n.buildPayload(body, title, notifyType, channel)
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		spec := RequestSpec{
			Method: "POST",
			URL:    notifiarrURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Content-Type": "application/json",
				"Accept":       "text/plain",
				"X-api-Key":    n.apikey,
			},
			Body: string(data),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (n *NotifiarrTarget) buildPayload(body, title string, notifyType NotifyType, channel int) map[string]any {
	mentions := n.extractMentions(body)

	source := n.source
	if source == "" {
		source = notifiarrAppID
	}

	event := ""
	update := false
	if n.event != 0 {
		event = strconv.Itoa(n.event)
		update = true
	}

	content := ""
	if len(mentions.content) > 0 {
		content = "👉 " + strings.Join(mentions.content, " ")
	}

	payload := map[string]any{
		"source": source,
		"type":   string(notifyType),
		"notification": map[string]any{
			"update": update,
			"name":   notifiarrAppID,
			"event":  event,
		},
		"discord": map[string]any{
			"color": appriseColor(notifyType),
			"ping": map[string]any{
				"pingUser": mentions.firstUser(),
				"pingRole": mentions.firstRole(),
			},
			"text": map[string]any{
				"title":       title,
				"content":     content,
				"description": body,
				"footer":      notifiarrAppDesc,
			},
			"ids": map[string]any{
				"channel": channel,
			},
		},
	}

	if n.includeImage {
		imageURL := appriseImageURL(notifyType, "256x256")
		payload["discord"].(map[string]any)["text"].(map[string]any)["icon"] = imageURL
		payload["discord"].(map[string]any)["images"] = map[string]any{
			"thumbnail": imageURL,
		}
	}

	return payload
}

type notifiarrMentions struct {
	users   []string
	roles   []string
	content []string
}

func (n *NotifiarrTarget) extractMentions(body string) notifiarrMentions {
	matches := notifiarrMentionRe.FindAllStringSubmatch(body, -1)
	mentions := notifiarrMentions{}
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		roleFlag := match[1]
		id := match[2]
		value := match[3]
		if value != "" {
			mentions.content = append(mentions.content, "@"+value)
			continue
		}
		if id == "" {
			continue
		}
		if roleFlag != "" {
			mentions.roles = append(mentions.roles, id)
			mentions.content = append(mentions.content, "<@&"+id+">")
		} else {
			mentions.users = append(mentions.users, id)
			mentions.content = append(mentions.content, "<@"+id+">")
		}
	}
	return mentions
}

func (m notifiarrMentions) firstUser() any {
	if len(m.users) == 0 {
		return 0
	}
	return m.users[0]
}

func (m notifiarrMentions) firstRole() any {
	if len(m.roles) == 0 {
		return 0
	}
	return m.roles[0]
}

func parseNotifiarrChannel(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	match := notifiarrChannelRe.FindStringSubmatch(raw)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func splitNotifiarrList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := notifiarrChannelDelims.Split(raw, -1)
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	return values
}

func normalizeNotifiarrChannels(values []string) []int {
	if len(values) == 0 {
		return nil
	}

	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		unique[value] = struct{}{}
	}

	sorted := make([]string, 0, len(unique))
	for value := range unique {
		sorted = append(sorted, value)
	}
	sort.Strings(sorted)

	channels := make([]int, 0, len(sorted))
	for _, value := range sorted {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		channels = append(channels, parsed)
	}

	return channels
}
