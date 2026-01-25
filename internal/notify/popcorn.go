package notify

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const popcornURL = "https://popcornnotify.com/notify"

var popcornKeyPattern = regexp.MustCompile(`(?i)^[a-z0-9]+$`)

var popcornListDelimiters = regexp.MustCompile(`[\[\];,\s]+`)

type PopcornTarget struct {
	apiKey  string
	batch   bool
	targets []string
}

func NewPopcornTarget(target *ParsedURL) (*PopcornTarget, error) {
	apiKey := strings.TrimSpace(target.Host)
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}
	if !popcornKeyPattern.MatchString(apiKey) {
		return nil, fmt.Errorf("invalid apikey")
	}

	targets := splitPath(target.Path)
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		targets = append(targets, parsePopcornList(toValue)...)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	batch := parseBool(target.Query["batch"], false)

	return &PopcornTarget{
		apiKey:  apiKey,
		batch:   batch,
		targets: targets,
	}, nil
}

func (p *PopcornTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(p.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	batchSize := 1
	if p.batch {
		batchSize = 10
	}
	if batchSize > len(p.targets) {
		batchSize = len(p.targets)
	}

	recipients := strings.Join(p.targets[:batchSize], ",")

	values := url.Values{}
	values.Set("message", body)
	values.Set("subject", title)
	values.Set("recipients", recipients)

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    popcornURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Accept":        "*/*",
			"Content-Type":  "application/x-www-form-urlencoded",
			"Authorization": basicAuthHeader(p.apiKey, "None"),
		},
		Body: values.Encode(),
	}, nil
}

func (p *PopcornTarget) Send(body, title string, notifyType NotifyType) error {
	if len(p.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	batchSize := 1
	if p.batch {
		batchSize = 10
	}

	for start := 0; start < len(p.targets); start += batchSize {
		end := start + batchSize
		if end > len(p.targets) {
			end = len(p.targets)
		}

		values := url.Values{}
		values.Set("message", body)
		values.Set("subject", title)
		values.Set("recipients", strings.Join(p.targets[start:end], ","))

		spec := RequestSpec{
			Method: "POST",
			URL:    popcornURL,
			Headers: map[string]string{
				"User-Agent":    "Apprise",
				"Accept":        "*/*",
				"Content-Type":  "application/x-www-form-urlencoded",
				"Authorization": basicAuthHeader(p.apiKey, "None"),
			},
			Body: values.Encode(),
		}

		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	return nil
}

func parsePopcornList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := popcornListDelimiters.Split(raw, -1)
	values := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values[part] = struct{}{}
	}

	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
