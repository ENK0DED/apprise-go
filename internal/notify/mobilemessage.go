package notify

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type MobileMessageTarget struct {
	deliveryTracker
	user, password, source, reference string
	unicode, batch                    bool
	maxParts                          int
	targets                           []string
}

var mobileMessageSenderPattern = regexp.MustCompile(`(?i)^\+?[a-z0-9][a-z0-9 ()_.-]{1,19}$`)
var mobileMessageNumericSender = regexp.MustCompile(`^\+?[0-9\s()-]+$`)
var mobileMessageAUPhone = regexp.MustCompile(`^614[0-9]{8}$`)

func NewMobileMessageTarget(p *ParsedURL) (*MobileMessageTarget, error) {
	t := &MobileMessageTarget{user: p.User, password: p.Password, source: p.Host, reference: p.Query["ref"], unicode: parseBoolWithDefault(p.Query["unicode"], false), batch: parseBoolWithDefault(p.Query["batch"], true), maxParts: 10}
	if t.user == "" || t.password == "" {
		return nil, fmt.Errorf("mobile Message requires username and password")
	}
	entries := splitPath(p.Path)
	if p.Query["from"] != "" {
		t.source = p.Query["from"]
		entries = append([]string{p.Host}, entries...)
	}
	if !mobileMessageSenderPattern.MatchString(t.source) {
		return nil, fmt.Errorf("invalid Mobile Message sender")
	}
	if mobileMessageNumericSender.MatchString(t.source) {
		var digits strings.Builder
		for _, r := range t.source {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		t.source = digits.String()
	}
	if raw := p.Query["max_parts"]; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 99 {
			return nil, fmt.Errorf("invalid Mobile Message part count")
		}
		t.maxParts = value
	}
	if fields := strings.Fields(t.reference); len(fields) > 0 {
		t.reference = fields[0]
	}
	entries = sortedUniqueTargets(append(entries, parseDelimitedList(p.Query["to"])...))
	for _, entry := range entries {
		number, ok := normalizePhoneWithBounds(entry, 9, 14)
		if !ok {
			continue
		}
		if strings.HasPrefix(number, "0") {
			number = "61" + number[1:]
		} else if len(number) == 9 {
			number = "61" + number
		}
		if mobileMessageAUPhone.MatchString(number) {
			t.targets = append(t.targets, number)
		}
	}
	return t, nil
}

func (t *MobileMessageTarget) request(body string, targets []string) (RequestSpec, error) {
	messages := make([]map[string]string, 0, len(targets))
	for _, target := range targets {
		entry := map[string]string{"to": target, "message": body, "sender": t.source}
		if t.reference != "" {
			entry["custom_ref"] = t.reference
		}
		messages = append(messages, entry)
	}
	data, err := json.Marshal(map[string]any{"messages": messages, "enable_unicode": t.unicode, "max_parts": t.maxParts})
	if err != nil {
		return RequestSpec{}, err
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(data))
	key := t.recall(fingerprint)
	if key == "" {
		key = newUUIDv4()
		t.remember(fingerprint, key)
	}
	return RequestSpec{Method: http.MethodPost, URL: "https://api.mobilemessage.com.au/v1/messages", Headers: map[string]string{"User-Agent": "Apprise", "Accept": "application/json", "Content-Type": "application/json", "Authorization": basicAuthHeader(t.user, t.password), "Idempotency-Key": key}, Body: string(data)}, nil
}

func (t *MobileMessageTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	if len(t.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("no Mobile Message recipients")
	}
	end := len(t.targets)
	if !t.batch {
		end = 1
	}
	return t.request(mergeTitleBody(title, body), t.targets[:end])
}

func (t *MobileMessageTarget) Send(body, title string, _ NotifyType) error {
	if len(t.targets) == 0 {
		return fmt.Errorf("no Mobile Message recipients")
	}
	batchSize := 10000
	if !t.batch {
		batchSize = 1
	}
	var outcome sendOutcome
	for start := 0; start < len(t.targets); start += batchSize {
		var targets []string
		for _, target := range t.targets[start:minInt(start+batchSize, len(t.targets))] {
			if !t.isDelivered(target) {
				targets = append(targets, target)
			}
		}
		if len(targets) == 0 {
			continue
		}
		spec, err := t.request(mergeTitleBody(title, body), targets)
		if err == nil {
			err = t.sendBatch(spec, targets)
		}
		outcome.record(err)
	}
	return outcome.err()
}

func (t *MobileMessageTarget) sendBatch(spec RequestSpec, targets []string) error {
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
	var result struct {
		Status  string            `json:"status"`
		Results []json.RawMessage `json:"results"`
	}
	if json.NewDecoder(response.Body).Decode(&result) != nil || result.Status != "complete" || len(result.Results) != len(targets) {
		return fmt.Errorf("mobile Message did not report every recipient")
	}
	var outcome sendOutcome
	for index, raw := range result.Results {
		var entry struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(raw, &entry) == nil && entry.Status == "success" {
			t.markDelivered(targets[index])
			outcome.record(nil)
		} else {
			outcome.record(fmt.Errorf("mobile Message rejected recipient"))
		}
	}
	return outcome.err()
}
