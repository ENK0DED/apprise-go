package notify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type GorushTarget struct {
	deliveryTracker
	endpoint, authorization, priority, topic, sound string
	platform                                        int
	badge                                           *int
	tokens                                          []string
	data                                            map[string]string
}

func NewGorushTarget(p *ParsedURL) (*GorushTarget, error) {
	t := &GorushTarget{platform: 2, topic: p.Query["topic"], sound: p.Query["sound"], data: p.QueryAdd}
	platform := p.Query["platform"]
	if platform == "" {
		platform = p.Query["mode"]
	}
	if platform != "" {
		t.platform = map[string]int{"ios": 1, "apns": 1, "1": 1, "android": 2, "fcm": 2, "2": 2, "huawei": 3, "hms": 3, "3": 3}[strings.ToLower(strings.TrimSpace(platform))]
		if t.platform == 0 {
			return nil, fmt.Errorf("invalid Gorush platform")
		}
	}
	var err error
	if p.Query["priority"] != "" {
		t.priority, err = incidentChoice(p.Query["priority"], "normal", "high")
		if err != nil {
			return nil, err
		}
	}
	if value := p.Query["badge"]; value != "" {
		badge, err := strconv.Atoi(value)
		if err != nil || badge < 0 {
			return nil, fmt.Errorf("invalid Gorush badge")
		}
		t.badge = &badge
	}
	t.tokens = sortedUniqueTargets(append(splitPath(p.Path), parseDelimitedList(p.Query["to"])...))
	host := p.Host
	if p.HasPort {
		host += ":" + strconv.Itoa(p.Port)
	}
	scheme := "http"
	if p.Scheme == "gorushs" {
		scheme = "https"
	}
	t.endpoint = scheme + "://" + host + "/api/push"
	if p.User != "" {
		password := p.Password
		if !p.HasPassword {
			password = "None"
		}
		t.authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(p.User+":"+password))
	}
	return t, nil
}

func (t *GorushTarget) requestForTokens(body, title string, tokens []string) (RequestSpec, error) {
	notification := map[string]any{"tokens": tokens, "platform": t.platform, "message": body}
	for key, value := range map[string]string{"title": title, "priority": t.priority, "topic": t.topic, "sound": t.sound} {
		if value != "" {
			notification[key] = value
		}
	}
	if t.badge != nil {
		notification["badge"] = *t.badge
	}
	if len(t.data) > 0 {
		notification["data"] = t.data
	}
	data, err := json.Marshal(map[string]any{"notifications": []any{notification}})
	headers := map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json"}
	if t.authorization != "" {
		headers["Authorization"] = t.authorization
	}
	return RequestSpec{Method: http.MethodPost, URL: t.endpoint, Headers: headers, Body: string(data)}, err
}

func (t *GorushTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	return t.requestForTokens(body, title, t.tokens)
}

func (t *GorushTarget) Send(body, title string, _ NotifyType) error {
	if len(t.tokens) == 0 {
		return fmt.Errorf("no Gorush devices")
	}
	var tokens []string
	for _, token := range t.tokens {
		if !t.isDelivered(token) {
			tokens = append(tokens, token)
		}
	}
	spec, err := t.requestForTokens(body, title, tokens)
	if err != nil {
		return err
	}
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	var content map[string]any
	if json.NewDecoder(response.Body).Decode(&content) != nil {
		return nil
	}
	failed := map[string]bool{}
	matchAll := true
	logs, _ := content["logs"].([]any)
	for _, raw := range logs {
		entry, ok := raw.(map[string]any)
		if !ok || entry["type"] != "failed-push" {
			continue
		}
		tokenValue, ok := entry["token"].(string)
		if !ok {
			matchAll = false
			failed[""] = true
			continue
		}
		failed[tokenValue] = true
		matched := false
		for _, token := range tokens {
			if token == tokenValue {
				matched = true
				break
			}
		}
		if !matched {
			matchAll = false
		}
	}
	if len(failed) == 0 {
		return nil
	}
	if matchAll {
		for _, token := range tokens {
			if !failed[token] {
				t.markDelivered(token)
			}
		}
	}
	return fmt.Errorf("gorush could not notify %d devices", len(failed))
}
