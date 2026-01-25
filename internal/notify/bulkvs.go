package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const bulkVSURL = "https://portal.bulkvs.com/api/v1.0/messageSend"
const bulkVSBatchSize = 4000

type BulkVSTarget struct {
	user     string
	password string
	source   string
	targets  []string
	batch    bool
}

func NewBulkVSTarget(target *ParsedURL) (*BulkVSTarget, error) {
	user := strings.TrimSpace(target.User)
	password := target.Password
	if user == "" || password == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	sourceRaw := ""
	targets := []string{}
	hasInvalid := false

	appendTarget := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if normalized, ok := normalizePhone(raw); ok {
			targets = append(targets, normalized)
			return
		}
		hasInvalid = true
	}

	if fromValue, ok := target.Query["from"]; ok && fromValue != "" {
		sourceRaw = fromValue
		appendTarget(target.Host)
		for _, entry := range splitPath(target.Path) {
			appendTarget(entry)
		}
	} else {
		sourceRaw = target.Host
		for _, entry := range splitPath(target.Path) {
			appendTarget(entry)
		}
	}

	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	source, ok := normalizePhone(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}

	if len(targets) == 0 && !hasInvalid {
		targets = append(targets, source)
	}

	batch := parseBool(target.Query["batch"], false)

	return &BulkVSTarget{
		user:     user,
		password: password,
		source:   source,
		targets:  targets,
		batch:    batch,
	}, nil
}

func (b *BulkVSTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(b.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	payload := map[string]any{
		"From":    b.source,
		"Message": message,
	}
	if b.batch {
		payload["To"] = b.targets[:minInt(len(b.targets), bulkVSBatchSize)]
	} else {
		payload["To"] = b.targets[0]
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    bulkVSURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "application/json",
			"Content-Type":  "application/json",
			"Authorization": basicAuthHeader(b.user, b.password),
		},
		Body: string(data),
	}, nil
}

func (b *BulkVSTarget) Send(body, title string, notifyType NotifyType) error {
	if len(b.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if b.batch {
		batchSize = bulkVSBatchSize
	}

	for index := 0; index < len(b.targets); index += batchSize {
		end := index + batchSize
		if end > len(b.targets) {
			end = len(b.targets)
		}
		payload := map[string]any{
			"From":    b.source,
			"Message": message,
		}
		if b.batch {
			payload["To"] = b.targets[index:end]
		} else {
			payload["To"] = b.targets[index]
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    bulkVSURL,
			Headers: map[string]string{
				"User-Agent":    "Apprise",
				"Accept":        "application/json",
				"Content-Type":  "application/json",
				"Authorization": basicAuthHeader(b.user, b.password),
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
