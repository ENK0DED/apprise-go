package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const mailgunBatchSize = 2000
const mailgunDefaultName = "Apprise"

var mailgunAPIBase = map[string]string{
	"us": "https://api.mailgun.net/v3/",
	"eu": "https://api.eu.mailgun.net/v3/",
}

type MailgunTarget struct {
	apiKey    string
	fromAddr  string
	fromName  string
	host      string
	region    string
	targets   []emailEntry
	cc        map[string]struct{}
	bcc       map[string]struct{}
	headers   map[string]string
	tokens    map[string]string
	batch     bool
}

func NewMailgunTarget(target *ParsedURL) (*MailgunTarget, error) {
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

	fromName := mailgunDefaultName
	if raw := strings.TrimSpace(target.Query["from"]); raw != "" {
		if isSimpleEmail(raw) {
			fromAddr = raw
			fromName = ""
		} else {
			fromName = raw
		}
	}
	if name := strings.TrimSpace(target.Query["name"]); name != "" && strings.TrimSpace(target.Query["from"]) == "" {
		fromName = name
	}

	region := strings.ToLower(strings.TrimSpace(target.Query["region"]))
	if region == "" {
		region = "us"
	}
	if _, ok := mailgunAPIBase[region]; !ok {
		return nil, fmt.Errorf("invalid region")
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
	if len(targets) == 0 {
		targets = []emailEntry{{name: fromName, email: fromAddr}}
	}

	cc := map[string]struct{}{}
	if ccValue, ok := target.Query["cc"]; ok && ccValue != "" {
		for _, entry := range parseDelimitedList(ccValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				cc[parsed.email] = struct{}{}
			}
		}
	}

	bcc := map[string]struct{}{}
	if bccValue, ok := target.Query["bcc"]; ok && bccValue != "" {
		for _, entry := range parseDelimitedList(bccValue) {
			if parsed, ok := parseEmailEntry(entry); ok {
				bcc[parsed.email] = struct{}{}
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

	return &MailgunTarget{
		apiKey:   apiKey,
		fromAddr: fromAddr,
		fromName: fromName,
		host:     host,
		region:   region,
		targets:  targets,
		cc:       cc,
		bcc:      bcc,
		headers:  headers,
		tokens:   tokens,
		batch:    parseBoolWithDefault(target.Query["batch"], false),
	}, nil
}

func (m *MailgunTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(m.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	batchSize := 1
	if m.batch {
		batchSize = mailgunBatchSize
	}

	payload := m.buildPayload(body, title, m.targets[:minInt(len(m.targets), batchSize)])
	encoded := payload.Encode()

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    m.buildURL(),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json",
			"Content-Type": "application/x-www-form-urlencoded",
			"Authorization": basicAuthHeader(
				"api",
				m.apiKey,
			),
		},
		Body: encoded,
	}, nil
}

func (m *MailgunTarget) Send(body, title string, notifyType NotifyType) error {
	if len(m.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	batchSize := 1
	if m.batch {
		batchSize = mailgunBatchSize
	}

	for index := 0; index < len(m.targets); index += batchSize {
		end := index + batchSize
		if end > len(m.targets) {
			end = len(m.targets)
		}

		payload := m.buildPayload(body, title, m.targets[index:end])
		spec := RequestSpec{
			Method: "POST",
			URL:    m.buildURL(),
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "application/json",
				"Content-Type": "application/x-www-form-urlencoded",
				"Authorization": basicAuthHeader(
					"api",
					m.apiKey,
				),
			},
			Body: payload.Encode(),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (m *MailgunTarget) buildPayload(body, title string, recipients []emailEntry) url.Values {
	values := url.Values{}
	values.Set("o:skip-verification", "False")
	values.Set("from", formatEmail(m.fromName, m.fromAddr))
	values.Set("subject", title)
	values.Set("html", body)

	toList := []string{}
	toEmails := map[string]struct{}{}
	for _, entry := range recipients {
		toList = append(toList, formatEmail(entry.name, entry.email))
		toEmails[entry.email] = struct{}{}
	}
	values.Set("to", strings.Join(toList, ","))

	cc := subtractEmailSet(m.cc, m.bcc, toEmails)
	if len(cc) > 0 {
		ccList := make([]string, 0, len(cc))
		for _, email := range cc {
			ccList = append(ccList, formatEmail("", email))
		}
		values.Set("cc", strings.Join(ccList, ","))
	}

	bcc := subtractEmailSet(m.bcc, nil, toEmails)
	if len(bcc) > 0 {
		values.Set("bcc", strings.Join(bcc, ","))
	}

	for key, value := range m.tokens {
		values.Set("v:"+key, value)
	}
	for key, value := range m.headers {
		values.Set("h:"+key, value)
	}

	return values
}

func (m *MailgunTarget) buildURL() string {
	return mailgunAPIBase[m.region] + m.host + "/messages"
}

func subtractEmailSet(source, remove map[string]struct{}, targets map[string]struct{}) []string {
	entries := []string{}
	for email := range source {
		if _, ok := targets[email]; ok {
			continue
		}
		if remove != nil {
			if _, ok := remove[email]; ok {
				continue
			}
		}
		entries = append(entries, email)
	}
	return entries
}
