package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type TelnyxTarget struct {
	deliveryTracker
	key, source, profile string
	targets              []string
}

func NewTelnyxTarget(p *ParsedURL) (*TelnyxTarget, error) {
	t := &TelnyxTarget{key: p.User, profile: p.Query["profile"]}
	if p.Query["key"] != "" {
		t.key = p.Query["key"]
	}
	if fields := strings.Fields(t.key); len(fields) > 0 {
		t.key = fields[0]
	} else {
		return nil, fmt.Errorf("missing Telnyx API key")
	}
	source := p.Host
	entries := splitPath(p.Path)
	if p.Query["from"] != "" {
		source = p.Query["from"]
		entries = append([]string{p.Host}, entries...)
	}
	var ok bool
	t.source, ok = normalizePhone(source)
	if !ok {
		return nil, fmt.Errorf("invalid Telnyx source phone")
	}
	entries = append(entries, parseDelimitedList(p.Query["to"])...)
	for _, entry := range sortedUniqueTargets(entries) {
		if number, valid := normalizePhone(entry); valid {
			t.targets = append(t.targets, number)
		}
	}
	if len(entries) == 0 {
		t.targets = []string{t.source}
	}
	if fields := strings.Fields(t.profile); len(fields) > 0 {
		t.profile = fields[0]
	}
	return t, nil
}

func (t *TelnyxTarget) requestForPhone(body, phone string) (RequestSpec, error) {
	payload := map[string]string{"from": "+" + t.source, "to": "+" + phone, "text": body}
	if t.profile != "" {
		payload["messaging_profile_id"] = t.profile
	}
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: "https://api.telnyx.com/v2/messages", Headers: map[string]string{"User-Agent": "Apprise", "Authorization": "Bearer " + t.key, "Accept": "application/json", "Content-Type": "application/json"}, Body: string(data)}, err
}

func (t *TelnyxTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	if len(t.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("no Telnyx recipients")
	}
	return t.requestForPhone(mergeTitleBody(title, body), t.targets[0])
}

func (t *TelnyxTarget) Send(body, title string, _ NotifyType) error {
	if len(t.targets) == 0 {
		return fmt.Errorf("no Telnyx recipients")
	}
	var outcome sendOutcome
	for _, phone := range t.targets {
		if t.isDelivered(phone) {
			continue
		}
		spec, err := t.requestForPhone(mergeTitleBody(title, body), phone)
		if err == nil {
			var req *http.Request
			req, err = spec.HTTPRequest()
			if err == nil {
				var response *http.Response
				response, err = httpClient().Do(req)
				if err == nil {
					response.Body.Close()
					if response.StatusCode != 200 {
						err = &HTTPStatusError{StatusCode: response.StatusCode}
					}
				}
			}
		}
		outcome.record(err)
		if err == nil {
			t.markDelivered(phone)
		}
	}
	return outcome.err()
}
