package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const sendgridURL = "https://api.sendgrid.com/v3/mail/send"
const sendgridDefaultSubject = "<no subject>"

type SendGridTarget struct {
	apiKey       string
	fromEmail    string
	targets      []string
	cc           map[string]struct{}
	bcc          map[string]struct{}
	templateID   string
	templateData map[string]string
}

func NewSendGridTarget(target *ParsedURL) (*SendGridTarget, error) {
	apiKey := strings.TrimSpace(target.User)
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	local := strings.TrimSpace(target.Password)
	host := strings.TrimSpace(target.Host)
	if local == "" || host == "" {
		return nil, fmt.Errorf("missing from email")
	}
	fromEmail := local + "@" + host
	if !isSimpleEmail(fromEmail) {
		return nil, fmt.Errorf("invalid from email")
	}

	targets := []string{}
	for _, entry := range splitPath(target.Path) {
		entry = strings.TrimSpace(entry)
		if isSimpleEmail(entry) {
			targets = append(targets, entry)
		}
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				targets = append(targets, entry)
			}
		}
	}
	if len(targets) == 0 {
		targets = append(targets, fromEmail)
	}

	cc := map[string]struct{}{}
	if ccValue, ok := target.Query["cc"]; ok && ccValue != "" {
		for _, entry := range parseDelimitedList(ccValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				cc[entry] = struct{}{}
			}
		}
	}

	bcc := map[string]struct{}{}
	if bccValue, ok := target.Query["bcc"]; ok && bccValue != "" {
		for _, entry := range parseDelimitedList(bccValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				bcc[entry] = struct{}{}
			}
		}
	}

	templateData := map[string]string{}
	for key, value := range target.QueryAdd {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		templateData[key] = value
	}

	return &SendGridTarget{
		apiKey:       apiKey,
		fromEmail:    fromEmail,
		targets:      targets,
		cc:           cc,
		bcc:          bcc,
		templateID:   strings.TrimSpace(target.Query["template"]),
		templateData: templateData,
	}, nil
}

func (s *SendGridTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	payload := s.buildPayload(body, title, s.targets[0])
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    sendgridURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
			"Authorization": fmt.Sprintf(
				"Bearer %s",
				s.apiKey,
			),
		},
		Body: string(data),
	}, nil
}

func (s *SendGridTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	for _, target := range s.targets {
		payload := s.buildPayload(body, title, target)
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    sendgridURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "*/*",
				"Content-Type": "application/json",
				"Authorization": fmt.Sprintf(
					"Bearer %s",
					s.apiKey,
				),
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

func (s *SendGridTarget) buildPayload(body, title, target string) map[string]any {
	subject := title
	if strings.TrimSpace(subject) == "" {
		subject = sendgridDefaultSubject
	}

	payload := map[string]any{
		"personalizations": []any{
			map[string]any{
				"to": []map[string]string{
					{"email": target},
				},
			},
		},
		"from": map[string]string{
			"email": s.fromEmail,
		},
		"subject": subject,
		"content": []map[string]string{
			{
				"type":  "text/html",
				"value": body,
			},
		},
	}

	cc := filterEmailSet(s.cc, s.bcc, target)
	if len(cc) > 0 {
		payload["personalizations"].([]any)[0].(map[string]any)["cc"] = toEmailObjects(cc)
	}

	bcc := filterEmailSet(s.bcc, nil, target)
	if len(bcc) > 0 {
		payload["personalizations"].([]any)[0].(map[string]any)["bcc"] = toEmailObjects(bcc)
	}

	if s.templateID != "" {
		payload["template_id"] = s.templateID
		if len(s.templateData) > 0 {
			payload["personalizations"].([]any)[0].(map[string]any)["dynamic_template_data"] = s.templateData
		}
	}

	return payload
}

func filterEmailSet(source, remove map[string]struct{}, target string) []string {
	filtered := []string{}
	for email := range source {
		if email == target {
			continue
		}
		if remove != nil {
			if _, ok := remove[email]; ok {
				continue
			}
		}
		filtered = append(filtered, email)
	}
	return filtered
}

func toEmailObjects(entries []string) []map[string]string {
	result := make([]map[string]string, 0, len(entries))
	for _, email := range entries {
		result = append(result, map[string]string{"email": email})
	}
	return result
}
