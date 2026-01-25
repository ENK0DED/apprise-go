package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const plivoURL = "https://api.plivo.com/v1/Account/{auth_id}/Message/"
const plivoBatchSize = 20

type PlivoTarget struct {
	authID  string
	token   string
	source  string
	targets []string
	batch   bool
}

func NewPlivoTarget(target *ParsedURL) (*PlivoTarget, error) {
	authID := strings.TrimSpace(target.User)
	if raw := strings.TrimSpace(target.Query["id"]); raw != "" {
		authID = raw
	}

	targets := splitPath(target.Path)

	token := strings.TrimSpace(target.Host)
	if raw := strings.TrimSpace(target.Query["token"]); raw != "" {
		token = raw
		if host := strings.TrimSpace(target.Host); host != "" {
			targets = append([]string{host}, targets...)
		}
	}

	if authID == "" || token == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	sourceRaw := strings.TrimSpace(target.Query["from"])
	if sourceRaw == "" {
		if len(targets) > 0 {
			sourceRaw = targets[0]
			targets = targets[1:]
		}
	}

	if sourceRaw == "" {
		return nil, fmt.Errorf("missing source")
	}
	sourceDigits, ok := normalizePhone(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}
	source := "+" + sourceDigits

	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			targets = append(targets, entry)
		}
	}

	normalizedTargets := []string{}
	for _, entry := range targets {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if normalized, ok := normalizePhone(entry); ok {
			normalizedTargets = append(normalizedTargets, "+"+normalized)
		}
	}

	if len(normalizedTargets) == 0 {
		normalizedTargets = []string{source}
	}

	batch := parseBoolWithDefault(target.Query["batch"], false)

	return &PlivoTarget{
		authID:  authID,
		token:   token,
		source:  source,
		targets: normalizedTargets,
		batch:   batch,
	}, nil
}

func (p *PlivoTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(p.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if p.batch {
		batchSize = plivoBatchSize
	}

	recipients := strings.Join(p.targets[:minInt(len(p.targets), batchSize)], ",")
	payload := map[string]any{
		"src":        p.source,
		"dst":        nil,
		"text":       message,
		"recipients": recipients,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	requestURL := plivoURL

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json",
			"Authorization": basicAuthHeader(
				p.authID,
				p.token,
			),
		},
		Body: string(data),
	}, nil
}

func (p *PlivoTarget) Send(body, title string, notifyType NotifyType) error {
	if len(p.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if p.batch {
		batchSize = plivoBatchSize
	}

	requestURL := plivoURL

	for index := 0; index < len(p.targets); index += batchSize {
		end := index + batchSize
		if end > len(p.targets) {
			end = len(p.targets)
		}

		payload := map[string]any{
			"src":        p.source,
			"dst":        nil,
			"text":       message,
			"recipients": strings.Join(p.targets[index:end], ","),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    requestURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "*/*",
				"Content-Type": "application/json",
				"Authorization": basicAuthHeader(
					p.authID,
					p.token,
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
