package notify

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const sparkpostDefaultRegion = "us"
const sparkpostDefaultSubject = "."
const sparkpostDefaultAppDesc = "Apprise Notifications"

var sparkpostAPIBase = map[string]string{
	"us": "https://api.sparkpost.com/api/v1",
	"eu": "https://api.eu.sparkpost.com/api/v1",
}

type SparkPostTarget struct {
	apiKey   string
	fromAddr string
	fromName string
	region   string
	targets  []emailEntry
	cc       map[string]struct{}
	bcc      map[string]struct{}
	names    map[string]string
	headers  map[string]string
	tokens   map[string]string
	batch    bool
}

func NewSparkPostTarget(target *ParsedURL) (*SparkPostTarget, error) {
	user := strings.TrimSpace(target.User)
	host := strings.TrimSpace(target.Host)
	if user == "" || host == "" {
		return nil, fmt.Errorf("missing sender")
	}
	fromAddr := user + "@" + host
	if !isSimpleEmail(fromAddr) {
		return nil, fmt.Errorf("invalid sender")
	}

	pathEntries := splitPath(target.Path)
	if len(pathEntries) == 0 {
		return nil, fmt.Errorf("missing apikey")
	}
	apiKey := strings.TrimSpace(pathEntries[0])
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	fromName := strings.TrimSpace(target.Query["name"])

	region := strings.ToLower(strings.TrimSpace(target.Query["region"]))
	if region == "" {
		region = sparkpostDefaultRegion
	}
	if _, ok := sparkpostAPIBase[region]; !ok {
		return nil, fmt.Errorf("invalid region")
	}

	targets := []emailEntry{}
	names := map[string]string{}
	for _, entry := range pathEntries[1:] {
		if parsed, ok := parseEmailEntry(entry); ok {
			targets = append(targets, parsed)
			if parsed.name != "" {
				names[parsed.email] = parsed.name
			}
		}
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				targets = append(targets, parsed)
				if parsed.name != "" {
					names[parsed.email] = parsed.name
				}
			}
		}
	}
	if len(targets) == 0 {
		targets = []emailEntry{{name: fromName, email: fromAddr}}
	}

	cc := map[string]struct{}{}
	if ccValue, ok := target.Query["cc"]; ok && ccValue != "" {
		for _, entry := range parseDelimitedList(ccValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				cc[parsed.email] = struct{}{}
				if parsed.name != "" {
					names[parsed.email] = parsed.name
				}
			}
		}
	}

	bcc := map[string]struct{}{}
	if bccValue, ok := target.Query["bcc"]; ok && bccValue != "" {
		for _, entry := range parseDelimitedList(bccValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				bcc[parsed.email] = struct{}{}
				if parsed.name != "" {
					names[parsed.email] = parsed.name
				}
			}
		}
	}

	headers := map[string]string{}
	for key, value := range target.QueryAdd {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		headers[key] = value
	}

	tokens := map[string]string{}
	for key, value := range target.QueryPayload {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		tokens[key] = value
	}

	return &SparkPostTarget{
		apiKey:   apiKey,
		fromAddr: fromAddr,
		fromName: fromName,
		region:   region,
		targets:  targets,
		cc:       cc,
		bcc:      bcc,
		names:    names,
		headers:  headers,
		tokens:   tokens,
		batch:    parseBoolWithDefault(target.Query["batch"], false),
	}, nil
}

func (s *SparkPostTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	batchSize := 1
	if s.batch {
		batchSize = 2000
	}

	payload := s.buildPayload(body, title, notifyType, s.targets[:minInt(len(s.targets), batchSize)])
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    s.buildURL(),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"Authorization": s.apiKey,
		},
		Body: string(data),
	}, nil
}

func (s *SparkPostTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	batchSize := 1
	if s.batch {
		batchSize = 2000
	}

	for index := 0; index < len(s.targets); index += batchSize {
		end := index + batchSize
		if end > len(s.targets) {
			end = len(s.targets)
		}

		payload := s.buildPayload(body, title, notifyType, s.targets[index:end])
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    s.buildURL(),
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "application/json",
				"Content-Type": "application/json",
				"Authorization": s.apiKey,
			},
			Body: string(data),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func (s *SparkPostTarget) buildPayload(body, title string, notifyType NotifyType, recipients []emailEntry) map[string]any {
	subject := strings.TrimSpace(title)
	if subject == "" {
		subject = sparkpostDefaultSubject
	}

	replyTo := formatEmail(s.fromName, s.fromAddr)
	fromName := s.fromName
	if fromName == "" {
		fromName = sparkpostDefaultAppDesc
	}

	payload := map[string]any{
		"options": map[string]any{
			"open_tracking":  false,
			"click_tracking": false,
		},
		"content": map[string]any{
			"from": map[string]string{
				"name":  fromName,
				"email": s.fromAddr,
			},
			"subject":  subject,
			"reply_to": replyTo,
			"html":     body,
		},
		"recipients":       []any{},
		"substitution_data": s.tokens,
	}

	headers := map[string]string{}
	for key, value := range s.headers {
		headers[key] = value
	}

	toEmails := make([]string, 0, len(recipients))
	recipientsList := make([]any, 0, len(recipients))
	for _, entry := range recipients {
		toEmails = append(toEmails, entry.email)
		address := map[string]any{
			"email": entry.email,
		}
		if entry.name != "" {
			address["name"] = entry.name
		}
		recipientsList = append(recipientsList, map[string]any{"address": address})
	}

	cc := subtractSets(s.cc, s.bcc, toEmails)
	bcc := subtractSets(s.bcc, nil, toEmails)

	if len(cc) > 0 {
		for _, email := range cc {
			entry := map[string]any{
				"address": map[string]any{
					"email":     email,
					"header_to": toEmails[0],
				},
			}
			if name, ok := s.names[email]; ok && name != "" {
				entry["address"].(map[string]any)["name"] = name
			}
			recipientsList = append(recipientsList, entry)
		}
		headers["CC"] = strings.Join(cc, ",")
	}

	if len(bcc) > 0 {
		for _, email := range bcc {
			recipientsList = append(recipientsList, map[string]any{
				"address": map[string]any{
					"email":     email,
					"header_to": toEmails[0],
				},
			})
		}
	}

	if len(headers) > 0 {
		payload["content"].(map[string]any)["headers"] = headers
	}

	payload["recipients"] = recipientsList

	_ = notifyType

	return payload
}

func (s *SparkPostTarget) buildURL() string {
	return sparkpostAPIBase[s.region] + "/transmissions/"
}

func subtractSets(source, remove map[string]struct{}, targets []string) []string {
	entries := []string{}
	targetSet := map[string]struct{}{}
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}

	for email := range source {
		if _, ok := targetSet[email]; ok {
			continue
		}
		if remove != nil {
			if _, ok := remove[email]; ok {
				continue
			}
		}
		entries = append(entries, email)
	}

	sort.Strings(entries)
	return entries
}
