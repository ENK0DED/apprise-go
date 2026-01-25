package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const d7NetworksURL = "https://api.d7networks.com/messages/v1/send"

type D7NetworksTarget struct {
	token   string
	targets []string
	source  string
	batch   bool
	unicode bool
}

func NewD7NetworksTarget(target *ParsedURL) (*D7NetworksTarget, error) {
	token := ""
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	} else if target.User != "" {
		token = target.User
		if target.Password != "" {
			token += ":" + target.Password
		}
	} else if target.Password != "" {
		token = ":" + target.Password
	}

	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	targets := []string{}
	appendTarget := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if normalized, ok := normalizePhone(raw); ok {
			targets = append(targets, normalized)
		}
	}

	appendTarget(target.Host)
	for _, entry := range splitPath(target.Path) {
		appendTarget(entry)
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	source := ""
	if rawSource, ok := target.Query["from"]; ok && rawSource != "" {
		source = rawSource
	} else if rawSource, ok := target.Query["source"]; ok && rawSource != "" {
		source = rawSource
	}
	source = strings.TrimSpace(source)

	batch := parseBool(target.Query["batch"], false)
	unicode := parseBool(target.Query["unicode"], false)

	return &D7NetworksTarget{
		token:   token,
		targets: targets,
		source:  source,
		batch:   batch,
		unicode: unicode,
	}, nil
}

func (d *D7NetworksTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(d.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	recipients := []string{d.targets[0]}
	if d.batch {
		recipients = d.targets
	}

	spec, err := d.buildRequest(recipients, message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (d *D7NetworksTarget) Send(body, title string, notifyType NotifyType) error {
	if len(d.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	if d.batch {
		spec, err := d.buildRequest(d.targets, message)
		if err != nil {
			return err
		}
		return SendRequest(spec)
	}

	for _, target := range d.targets {
		spec, err := d.buildRequest([]string{target}, message)
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

func (d *D7NetworksTarget) buildRequest(recipients []string, message string) (RequestSpec, error) {
	dataCoding := "auto"
	if d.unicode {
		dataCoding = "unicode"
	}

	messageGlobals := map[string]any{
		"channel": "sms",
	}
	if d.source != "" {
		messageGlobals["originator"] = d.source
	}

	payload := map[string]any{
		"message_globals": messageGlobals,
		"messages": []map[string]any{
			{
				"recipients":  recipients,
				"content":     message,
				"data_coding": dataCoding,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    d7NetworksURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "application/json",
			"Content-Type":  "application/json",
			"Authorization": fmt.Sprintf("Bearer %s", d.token),
		},
		Body: string(data),
	}, nil
}
