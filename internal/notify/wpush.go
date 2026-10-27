package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type WPushTarget struct {
	deliveryTracker
	apiKey, channel, group, clickURL string
	topics                           []string
}

var wpushKeyPattern = regexp.MustCompile(`(?i)^WPUSH[a-z0-9]{27}$`)

func NewWPushTarget(p *ParsedURL) (*WPushTarget, error) {
	t := &WPushTarget{apiKey: p.Host, group: p.Query["option"], clickURL: p.Query["url"]}
	if p.Query["apikey"] != "" {
		t.apiKey = p.Query["apikey"]
	}
	if !wpushKeyPattern.MatchString(t.apiKey) {
		return nil, fmt.Errorf("invalid WPUSH API key")
	}
	channels := parseDelimitedList(strings.ToLower(p.Query["channel"]))
	if len(channels) == 0 {
		channels = []string{"wechat"}
	}
	allowed := map[string]bool{"wechat": true, "app": true, "sms": true, "mail": true, "webhook": true, "dingtalk": true, "feishu": true, "wechat_work": true, "clawbot": true, "qqbot": true}
	for _, channel := range channels {
		if !allowed[channel] {
			return nil, fmt.Errorf("invalid WPUSH channel: %s", channel)
		}
	}
	t.channel = strings.Join(sortedUniqueTargets(channels), ",")
	t.topics = splitPath(p.Path)
	for _, key := range []string{"to", "topic_code", "topic"} {
		t.topics = append(t.topics, parseDelimitedList(p.Query[key])...)
	}
	t.topics = sortedUniqueTargets(t.topics)
	if p.Query["group"] != "" {
		t.group = p.Query["group"]
	}
	if strings.TrimSpace(t.group) == "" {
		t.group = ""
	}
	if t.group != "" && len(t.topics) > 0 {
		return nil, fmt.Errorf("WPUSH group cannot be combined with topics")
	}
	return t, nil
}

func (t *WPushTarget) requestForTopic(body, title, topic string) (RequestSpec, error) {
	if title == "" {
		title = body
	}
	payload := map[string]string{"apikey": t.apiKey, "title": title, "content": body, "channel": t.channel}
	if topic != "" {
		payload["topic_code"] = topic
	} else if t.group != "" {
		payload["option"] = t.group
	}
	if strings.TrimSpace(t.clickURL) != "" {
		payload["url"] = t.clickURL
	}
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: "https://api.wpush.cn/api/v1/send", Headers: map[string]string{
		"User-Agent": "Apprise", "Content-Type": "application/json", "X-API-Key": t.apiKey, "X-Idempotency-Key": newUUIDv4(),
	}, Body: string(data)}, err
}

func (t *WPushTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	topic := ""
	if len(t.topics) > 0 {
		topic = t.topics[0]
	}
	return t.requestForTopic(body, title, topic)
}

func (t *WPushTarget) Send(body, title string, _ NotifyType) error {
	topics := t.topics
	if len(topics) == 0 {
		topics = []string{""}
	}
	var outcome sendOutcome
	for _, topic := range topics {
		if t.isDelivered(topic) {
			continue
		}
		err := t.sendTopic(body, title, topic)
		outcome.record(err)
		if err == nil {
			t.markDelivered(topic)
		}
	}
	return outcome.err()
}

func (t *WPushTarget) sendTopic(body, title, topic string) error {
	spec, err := t.requestForTopic(body, title, topic)
	if err != nil {
		return err
	}
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	// WPUSH sends the API key in both its headers and JSON body. A redirect
	// must not forward either credential to another endpoint. Use a local
	// client option so this protection does not alter other providers.
	currentHTTPOptionsMu.RLock()
	options := currentHTTPOptions
	currentHTTPOptionsMu.RUnlock()
	options.followRedirect = false
	response, err := httpClientForOptions(options).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	var result struct {
		Code *float64 `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("invalid WPUSH response: %w", err)
	}
	if result.Code == nil || *result.Code != 0 {
		return fmt.Errorf("WPUSH rejected notification")
	}
	return nil
}
