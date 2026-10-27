package notify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type oneBotRecipient struct{ field, id, action string }

type OneBotTarget struct {
	deliveryTracker
	endpoint, token string
	recipients      []oneBotRecipient
}

var oneBotTargetPattern = regexp.MustCompile(`^([@#]?)([0-9]{1,19})$`)

func NewOneBotTarget(p *ParsedURL) (*OneBotTarget, error) {
	if !isValidHostname(p.Host) {
		return nil, fmt.Errorf("invalid OneBot hostname")
	}
	t := &OneBotTarget{token: p.User}
	if p.Query["token"] != "" {
		t.token = p.Query["token"]
	}
	scheme := "http"
	if p.Scheme == "onebots" {
		scheme = "https"
	}
	t.endpoint = scheme + "://" + p.Host
	if p.HasPort {
		t.endpoint += ":" + strconv.Itoa(p.Port)
	}
	entries := sortedUniqueTargets(append(splitPath(p.Path), parseDelimitedList(p.Query["to"])...))
	var users, groups []oneBotRecipient
	for _, entry := range entries {
		match := oneBotTargetPattern.FindStringSubmatch(entry)
		if match == nil {
			continue
		}
		if match[1] == "#" {
			groups = append(groups, oneBotRecipient{"group_id", match[2], "send_group_msg"})
		} else {
			users = append(users, oneBotRecipient{"user_id", match[2], "send_private_msg"})
		}
	}
	t.recipients = append(users, groups...)
	return t, nil
}

func (t *OneBotTarget) request(recipient oneBotRecipient, segment map[string]any) (RequestSpec, error) {
	id, err := strconv.ParseUint(recipient.id, 10, 64)
	if err != nil {
		return RequestSpec{}, err
	}
	data, err := json.Marshal(map[string]any{recipient.field: id, "message": []any{segment}})
	headers := map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json"}
	if t.token != "" {
		headers["Authorization"] = "Bearer " + t.token
	}
	return RequestSpec{Method: http.MethodPost, URL: t.endpoint + "/" + recipient.action, Headers: headers, Body: string(data)}, err
}

func (t *OneBotTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	if len(t.recipients) == 0 {
		return RequestSpec{}, fmt.Errorf("no OneBot recipients")
	}
	return t.request(t.recipients[0], map[string]any{"type": "text", "data": map[string]string{"text": mergeTitleBody(title, body)}})
}

func (t *OneBotTarget) Send(body, title string, kind NotifyType) error {
	return t.SendWithAttachments(body, title, kind, nil)
}

func (t *OneBotTarget) SendWithAttachments(body, title string, _ NotifyType, attachments []Attachment) error {
	if len(t.recipients) == 0 {
		return fmt.Errorf("no OneBot recipients")
	}
	body = mergeTitleBody(title, body)
	failed := map[string]bool{}
	var outcome sendOutcome
	send := func(recipient oneBotRecipient, segment map[string]any, deliveryKey string) {
		key := recipient.field + ":" + recipient.id
		if failed[key] || t.isDelivered(deliveryKey) {
			return
		}
		spec, err := t.request(recipient, segment)
		if err == nil {
			err = sendOneBotRequest(spec)
		}
		outcome.record(err)
		if err == nil {
			t.markDelivered(deliveryKey)
		} else {
			failed[key] = true
		}
	}
	if body != "" {
		for _, recipient := range t.recipients {
			send(recipient, map[string]any{"type": "text", "data": map[string]string{"text": body}}, recipient.field+":"+recipient.id)
		}
	}
	for index, attachment := range attachments {
		kind := "file"
		for prefix, value := range map[string]string{"image/": "image", "audio/": "record", "video/": "video"} {
			if strings.HasPrefix(attachment.MIMEType, prefix) {
				kind = value
				break
			}
		}
		data := map[string]string{"file": "base64://" + base64.StdEncoding.EncodeToString(attachment.Data)}
		if kind == "file" {
			data["name"] = attachment.Name
			if data["name"] == "" {
				data["name"] = "attachment.dat"
			}
		}
		segment := map[string]any{"type": kind, "data": data}
		for _, recipient := range t.recipients {
			send(recipient, segment, fmt.Sprintf("attachment:%s:%s:%d", recipient.field, recipient.id, index))
		}
	}
	return outcome.err()
}

func sendOneBotRequest(spec RequestSpec) error {
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
	if content["status"] == "failed" {
		return fmt.Errorf("OneBot action failed")
	}
	if code, exists := content["retcode"]; exists {
		switch value := code.(type) {
		case float64:
			if value == 0 || value == 1 {
				return nil
			}
		case bool:
			return nil
		}
		return fmt.Errorf("OneBot rejected action: retcode=%v", code)
	}
	return nil
}
