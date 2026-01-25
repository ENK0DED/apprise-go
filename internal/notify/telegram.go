package notify

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

const telegramAPIBase = "https://api.telegram.org/bot"

type telegramRecipient struct {
	chatID       string
	chatIDInt    int64
	isNumeric    bool
	messageTopic int
}

type TelegramTarget struct {
	botToken string
	targets  []telegramRecipient
	silent   bool
	preview  bool
}

func NewTelegramTarget(target *ParsedURL) (*TelegramTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing bot token")
	}

	segments := splitPath(target.Path)
	decodedHost, err := url.PathUnescape(host)
	if err != nil {
		decodedHost = host
	}

	botToken := ""
	rawTargets := []string{}
	if strings.Contains(decodedHost, ":") {
		botToken = decodedHost
		rawTargets = append(rawTargets, segments...)
	} else {
		if len(segments) == 0 {
			return nil, fmt.Errorf("missing bot token")
		}
		botToken = decodedHost + ":" + segments[0]
		rawTargets = append(rawTargets, segments[1:]...)
	}

	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		rawTargets = append(rawTargets, parseDelimitedList(toValue)...)
	}

	defaultTopic := parseOptionalIntValue(target.Query["topic"])
	if defaultTopic == nil {
		defaultTopic = parseOptionalIntValue(target.Query["thread"])
	}

	targets := make([]telegramRecipient, 0, len(rawTargets))
	for _, entry := range rawTargets {
		if recipient, ok := parseTelegramRecipient(entry, defaultTopic); ok {
			targets = append(targets, recipient)
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	return &TelegramTarget{
		botToken: botToken,
		targets:  targets,
		silent:   parseBoolValue(target.Query["silent"], false),
		preview:  parseBoolValue(target.Query["preview"], false),
	}, nil
}

func (t *TelegramTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(t.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := formatTelegramMessage(title, body)
	spec, err := t.buildSpec(message, t.targets[0])
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (t *TelegramTarget) Send(body, title string, notifyType NotifyType) error {
	if len(t.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := formatTelegramMessage(title, body)
	for _, recipient := range t.targets {
		spec, err := t.buildSpec(message, recipient)
		if err != nil {
			return err
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (t *TelegramTarget) buildSpec(body string, recipient telegramRecipient) (RequestSpec, error) {
	payload := map[string]any{
		"disable_notification":     t.silent,
		"disable_web_page_preview": !t.preview,
		"parse_mode":               "HTML",
		"text":                     body,
	}

	if recipient.isNumeric {
		payload["chat_id"] = recipient.chatIDInt
	} else {
		payload["chat_id"] = recipient.chatID
	}
	if recipient.messageTopic > 0 {
		payload["message_thread_id"] = recipient.messageTopic
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    telegramAPIBase + t.botToken + "/sendMessage",
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func parseTelegramRecipient(raw string, defaultTopic *int) (telegramRecipient, bool) {
	entry := strings.TrimSpace(raw)
	if entry == "" {
		return telegramRecipient{}, false
	}

	topic := 0
	if defaultTopic != nil {
		topic = *defaultTopic
	}

	base, parsedTopic, ok := splitTelegramTopic(entry)
	if ok {
		entry = base
		topic = parsedTopic
	}

	entry = strings.TrimSpace(entry)
	if entry == "" {
		return telegramRecipient{}, false
	}

	if id, err := strconv.ParseInt(entry, 10, 64); err == nil {
		return telegramRecipient{
			chatID:       entry,
			chatIDInt:    id,
			isNumeric:    true,
			messageTopic: topic,
		}, true
	}

	if strings.HasPrefix(entry, "@") {
		return telegramRecipient{chatID: entry, messageTopic: topic}, true
	}

	return telegramRecipient{chatID: "@" + entry, messageTopic: topic}, true
}

func splitTelegramTopic(entry string) (string, int, bool) {
	parts := strings.SplitN(entry, ":", 2)
	if len(parts) != 2 {
		return entry, 0, false
	}
	base := strings.TrimSpace(parts[0])
	if base == "" {
		return entry, 0, false
	}
	value := strings.TrimSpace(parts[1])
	if value == "" {
		return entry, 0, false
	}
	topic, err := strconv.Atoi(value)
	if err != nil {
		return entry, 0, false
	}
	return base, topic, true
}

func parseBoolValue(raw string, fallback bool) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "1", "true", "yes", "on", "y":
		return true
	case "0", "false", "no", "off", "n":
		return false
	default:
		return fallback
	}
}

func parseOptionalIntValue(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func formatTelegramMessage(title, body string) string {
	if title == "" {
		return html.EscapeString(body)
	}
	escapedTitle := html.EscapeString(title)
	if body == "" {
		return "<b>" + escapedTitle + "</b>"
	}
	return "<b>" + escapedTitle + "</b>\r\n" + html.EscapeString(body)
}
