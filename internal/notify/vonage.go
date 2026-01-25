package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const vonageURL = "https://rest.nexmo.com/sms/json"
const vonageDefaultTTL = 900000
const vonageMinTTL = 20000
const vonageMaxTTL = 604800000

type VonageTarget struct {
	apiKey  string
	secret  string
	source  string
	ttl     int
	targets []string
}

func NewVonageTarget(target *ParsedURL) (*VonageTarget, error) {
	apiKey := strings.TrimSpace(target.User)
	secret := target.Password
	if raw := strings.TrimSpace(target.Query["key"]); raw != "" {
		apiKey = raw
	}
	if raw := strings.TrimSpace(target.Query["secret"]); raw != "" {
		secret = raw
	}
	if apiKey == "" || secret == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	ttl := vonageDefaultTTL
	if raw := strings.TrimSpace(target.Query["ttl"]); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			ttl = value
		}
	}
	if ttl < vonageMinTTL || ttl > vonageMaxTTL {
		return nil, fmt.Errorf("invalid ttl")
	}

	sourceRaw := strings.TrimSpace(target.Host)
	if raw := strings.TrimSpace(target.Query["from"]); raw != "" {
		sourceRaw = raw
	} else if raw := strings.TrimSpace(target.Query["source"]); raw != "" {
		sourceRaw = raw
	}
	if sourceRaw == "" {
		return nil, fmt.Errorf("missing source")
	}
	source, ok := normalizePhone(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
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

	for _, entry := range splitPath(target.Path) {
		appendTarget(entry)
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			appendTarget(entry)
		}
	}

	return &VonageTarget{
		apiKey:  apiKey,
		secret:  secret,
		source:  source,
		ttl:     ttl,
		targets: targets,
	}, nil
}

func (v *VonageTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	targets := v.targets
	if len(targets) == 0 {
		targets = []string{v.source}
	}
	if len(targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)

	values := url.Values{}
	values.Set("api_key", v.apiKey)
	values.Set("api_secret", v.secret)
	values.Set("ttl", strconv.Itoa(v.ttl))
	values.Set("from", v.source)
	values.Set("text", message)
	values.Set("to", targets[0])

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    vonageURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: values.Encode(),
	}, nil
}

func (v *VonageTarget) Send(body, title string, notifyType NotifyType) error {
	targets := v.targets
	if len(targets) == 0 {
		targets = []string{v.source}
	}
	if len(targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)

	for _, target := range targets {
		values := url.Values{}
		values.Set("api_key", v.apiKey)
		values.Set("api_secret", v.secret)
		values.Set("ttl", strconv.Itoa(v.ttl))
		values.Set("from", v.source)
		values.Set("text", message)
		values.Set("to", target)

		spec := RequestSpec{
			Method: "POST",
			URL:    vonageURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "*/*",
				"Content-Type": "application/x-www-form-urlencoded",
			},
			Body: values.Encode(),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}
