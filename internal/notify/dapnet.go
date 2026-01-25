package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const dapnetURL = "http://www.hampager.de:8080/calls"
const dapnetBatchSize = 50

const (
	dapnetPriorityNormal = iota
	dapnetPriorityEmergency
)

var dapnetCallSignRe = regexp.MustCompile(`(?i)^([0-9a-z]{1,2}[0-9][a-z0-9]{1,3})(?:-([0-9]{1,2}))?\s*$`)

type DapnetTarget struct {
	user     string
	password string
	priority int
	txgroups []string
	batch    bool
	targets  []string
}

type dapnetPayload struct {
	Text                  string   `json:"text"`
	CallSignNames         []string `json:"callSignNames"`
	TransmitterGroupNames []string `json:"transmitterGroupNames"`
	Emergency             bool     `json:"emergency"`
}

func NewDapnetTarget(target *ParsedURL) (*DapnetTarget, error) {
	user := strings.TrimSpace(target.User)
	password := target.Password
	if user == "" || password == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	priority := parseDapnetPriority(target.Query["priority"])
	txgroups := parseDapnetTxGroups(target.Query["txgroups"])
	batch := parseBoolWithDefault(target.Query["batch"], false)

	entries := []string{}
	if target.Host != "" {
		entries = append(entries, target.Host)
	}
	entries = append(entries, splitPath(target.Path)...)
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}

	targets := []string{}
	seen := map[string]struct{}{}
	for _, entry := range entries {
		callSign, ok := normalizeDapnetCallSign(entry)
		if !ok {
			continue
		}
		if _, exists := seen[callSign]; exists {
			continue
		}
		seen[callSign] = struct{}{}
		targets = append(targets, callSign)
	}

	return &DapnetTarget{
		user:     user,
		password: password,
		priority: priority,
		txgroups: txgroups,
		batch:    batch,
		targets:  targets,
	}, nil
}

func (d *DapnetTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(d.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if d.batch {
		batchSize = dapnetBatchSize
	}
	chunk := d.targets
	if len(chunk) > batchSize {
		chunk = chunk[:batchSize]
	}

	spec, err := d.buildRequest(chunk, message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (d *DapnetTarget) Send(body, title string, notifyType NotifyType) error {
	if len(d.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if d.batch {
		batchSize = dapnetBatchSize
	}

	for idx := 0; idx < len(d.targets); idx += batchSize {
		end := idx + batchSize
		if end > len(d.targets) {
			end = len(d.targets)
		}
		spec, err := d.buildRequest(d.targets[idx:end], message)
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

func (d *DapnetTarget) buildRequest(targets []string, message string) (RequestSpec, error) {
	payload := dapnetPayload{
		Text:                  message,
		CallSignNames:         targets,
		TransmitterGroupNames: d.txgroups,
		Emergency:             d.priority == dapnetPriorityEmergency,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    dapnetURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/json; charset=utf-8",
			"Authorization": basicAuthHeader(d.user, d.password),
		},
		Body: string(data),
	}, nil
}

func parseDapnetPriority(raw string) int {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return dapnetPriorityNormal
	}
	priorityMap := []struct {
		prefix string
		value  int
	}{
		{"n", dapnetPriorityNormal},
		{"e", dapnetPriorityEmergency},
		{"0", dapnetPriorityNormal},
		{"1", dapnetPriorityEmergency},
	}
	for _, entry := range priorityMap {
		if strings.HasPrefix(raw, entry.prefix) {
			return entry.value
		}
	}
	return dapnetPriorityNormal
}

func parseDapnetTxGroups(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"dl-all"}
	}

	entries := parseDelimitedList(raw)
	if len(entries) == 0 {
		return []string{"dl-all"}
	}

	groups := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		groups = append(groups, strings.ToLower(entry))
	}
	if len(groups) == 0 {
		return []string{"dl-all"}
	}
	return groups
}

func normalizeDapnetCallSign(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	match := dapnetCallSignRe.FindStringSubmatch(raw)
	if match == nil {
		return "", false
	}
	return strings.ToUpper(match[1]), true
}
