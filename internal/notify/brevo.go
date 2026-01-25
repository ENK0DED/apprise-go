package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const brevoURL = "https://api.brevo.com/v3/smtp/email"
const brevoDefaultSubject = "<no subject>"

type BrevoTarget struct {
	apiKey    string
	fromEmail string
	targets   []string
	cc        map[string]struct{}
	bcc       map[string]struct{}
	replyTo   string
}

func NewBrevoTarget(target *ParsedURL) (*BrevoTarget, error) {
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

	replyTo := strings.TrimSpace(target.Query["reply"])

	return &BrevoTarget{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		targets:   targets,
		cc:        cc,
		bcc:       bcc,
		replyTo:   replyTo,
	}, nil
}

func (b *BrevoTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(b.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	payload := b.buildPayload(body, title, b.targets[0])
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    brevoURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"api-key":      b.apiKey,
		},
		Body: string(data),
	}, nil
}

func (b *BrevoTarget) Send(body, title string, notifyType NotifyType) error {
	if len(b.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	for _, target := range b.targets {
		payload := b.buildPayload(body, title, target)
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    brevoURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "application/json",
				"Content-Type": "application/json",
				"api-key":      b.apiKey,
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

func (b *BrevoTarget) buildPayload(body, title, target string) map[string]any {
	subject := title
	if strings.TrimSpace(subject) == "" {
		subject = brevoDefaultSubject
	}

	payload := map[string]any{
		"sender": map[string]string{
			"email": b.fromEmail,
		},
		"to": []map[string]string{
			{"email": target},
		},
		"subject":     subject,
		"htmlContent": body,
		"textContent": body,
	}

	cc := filterEmailSet(b.cc, b.bcc, target)
	if len(cc) > 0 {
		payload["cc"] = toEmailObjects(cc)
	}

	bcc := filterEmailSet(b.bcc, nil, target)
	if len(bcc) > 0 {
		payload["bcc"] = toEmailObjects(bcc)
	}

	if b.replyTo != "" && isSimpleEmail(b.replyTo) {
		payload["replyTo"] = map[string]string{
			"email": b.replyTo,
		}
	}

	return payload
}
