package notify

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const smtp2goURL = "https://api.smtp2go.com/v3/email/send"
const smtp2goBatchSize = 100

var smtp2goEmailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type SMTP2GoTarget struct {
	apiKey   string
	fromAddr string
	fromName string
	targets  []emailEntry
	cc       []emailEntry
	bcc      []string
	headers  map[string]string
	batch    bool
}

type emailEntry struct {
	name  string
	email string
}

func NewSMTP2GoTarget(target *ParsedURL) (*SMTP2GoTarget, error) {
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

	targets := []emailEntry{}
	for _, entry := range pathEntries[1:] {
		if parsed, ok := parseEmailEntry(entry); ok {
			targets = append(targets, parsed)
		}
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				targets = append(targets, parsed)
			}
		}
	}

	fromName := strings.TrimSpace(target.Query["name"])

	cc := []emailEntry{}
	if ccValue, ok := target.Query["cc"]; ok && ccValue != "" {
		for _, entry := range parseDelimitedList(ccValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				cc = append(cc, parsed)
			}
		}
	}

	bcc := []string{}
	if bccValue, ok := target.Query["bcc"]; ok && bccValue != "" {
		for _, entry := range parseDelimitedList(bccValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				bcc = append(bcc, entry)
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

	batch := parseBoolWithDefault(target.Query["batch"], false)

	if len(targets) == 0 {
		targets = []emailEntry{{name: fromName, email: fromAddr}}
	}

	return &SMTP2GoTarget{
		apiKey:   apiKey,
		fromAddr: fromAddr,
		fromName: fromName,
		targets:  targets,
		cc:       cc,
		bcc:      bcc,
		headers:  headers,
		batch:    batch,
	}, nil
}

func (s *SMTP2GoTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	batchSize := 1
	if s.batch {
		batchSize = smtp2goBatchSize
	}

	payload := s.buildPayload(body, title, s.targets[:minInt(len(s.targets), batchSize)])
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    smtp2goURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (s *SMTP2GoTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	batchSize := 1
	if s.batch {
		batchSize = smtp2goBatchSize
	}

	for index := 0; index < len(s.targets); index += batchSize {
		end := index + batchSize
		if end > len(s.targets) {
			end = len(s.targets)
		}

		payload := s.buildPayload(body, title, s.targets[index:end])
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    smtp2goURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "application/json",
				"Content-Type": "application/json",
			},
			Body: string(data),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (s *SMTP2GoTarget) buildPayload(body, title string, recipients []emailEntry) map[string]any {
	payload := map[string]any{
		"api_key":  s.apiKey,
		"sender":   formatEmail(s.fromName, s.fromAddr),
		"subject":  title,
		"to":       []string{},
		"html_body": body,
	}

	to := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		to = append(to, formatEmail(recipient.name, recipient.email))
	}
	payload["to"] = to

	if len(s.cc) > 0 {
		cc := make([]string, 0, len(s.cc))
		for _, recipient := range s.cc {
			cc = append(cc, formatEmail(recipient.name, recipient.email))
		}
		payload["cc"] = cc
	}

	if len(s.bcc) > 0 {
		payload["bcc"] = s.bcc
	}

	if len(s.headers) > 0 {
		customHeaders := make([]map[string]string, 0, len(s.headers))
		for key, value := range s.headers {
			customHeaders = append(customHeaders, map[string]string{
				"header": key,
				"value":  value,
			})
		}
		payload["custom_headers"] = customHeaders
	}

	return payload
}

func parseEmailEntry(raw string) (emailEntry, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return emailEntry{}, false
	}

	name := ""
	email := trimmed
	if parts := strings.SplitN(trimmed, ":", 2); len(parts) == 2 {
		name = strings.TrimSpace(parts[0])
		email = strings.TrimSpace(parts[1])
	}

	if !isSimpleEmail(email) {
		return emailEntry{}, false
	}

	return emailEntry{name: name, email: email}, true
}

func formatEmail(name, email string) string {
	if name == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func isSimpleEmail(value string) bool {
	return smtp2goEmailRegex.MatchString(value)
}
