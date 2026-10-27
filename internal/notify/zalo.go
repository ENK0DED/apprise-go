package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

type ZaloTarget struct {
	deliveryTracker
	target *ParsedURL
	token  string
	chats  []string
}

var zaloTokenPattern = regexp.MustCompile(`(?i)^[0-9]+:[a-z0-9_-]+$`)
var zaloAuthorityPrefix = regexp.MustCompile(`(?i)^(zalo://)([0-9]+)(?::|%3a)`)
var zaloChatPattern = regexp.MustCompile(`(?i)^[a-z0-9._-]+$`)

func NewZaloTarget(p *ParsedURL) (*ZaloTarget, error) {
	t := &ZaloTarget{target: p, token: p.Query["token"]}
	entries := splitPath(p.Path)
	if t.token != "" {
		entries = append([]string{p.Host}, entries...)
	} else {
		if len(entries) == 0 {
			return nil, fmt.Errorf("missing Zalo token")
		}
		t.token = p.Host + ":" + entries[0]
		entries = entries[1:]
	}
	if !zaloTokenPattern.MatchString(t.token) {
		return nil, fmt.Errorf("invalid Zalo token")
	}
	entries = sortedUniqueTargets(append(entries, parseDelimitedList(p.Query["to"])...))
	for _, entry := range entries {
		if zaloChatPattern.MatchString(entry) {
			t.chats = append(t.chats, entry)
		}
	}
	return t, nil
}

func (t *ZaloTarget) request(body, title, chat string) (RequestSpec, error) {
	payload := map[string]string{"text": mergeTitleBody(title, body), "chat_id": chat}
	if targetNotifyFormat(t.target) == "markdown" {
		payload["parse_mode"] = "markdown"
	}
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: "https://bot-api.zaloplatforms.com/bot" + t.token + "/sendMessage", Headers: map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json; charset=utf-8"}, Body: string(data)}, err
}

func (t *ZaloTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	if len(t.chats) == 0 {
		return RequestSpec{}, fmt.Errorf("no Zalo chats")
	}
	return t.request(body, title, t.chats[0])
}

func (t *ZaloTarget) Send(body, title string, _ NotifyType) error {
	if len(t.chats) == 0 {
		return fmt.Errorf("no Zalo chats")
	}
	var outcome sendOutcome
	for _, chat := range t.chats {
		if t.isDelivered(chat) {
			continue
		}
		spec, err := t.request(body, title, chat)
		if err == nil {
			err = sendZaloRequest(spec)
		}
		outcome.record(err)
		if err == nil {
			t.markDelivered(chat)
		}
	}
	return outcome.err()
}

func sendZaloRequest(spec RequestSpec) error {
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	var content map[string]any
	if json.NewDecoder(response.Body).Decode(&content) == nil && content["ok"] == false {
		return fmt.Errorf("zalo rejected message")
	}
	return nil
}
