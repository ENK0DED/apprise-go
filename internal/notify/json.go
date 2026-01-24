package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var jsonMethods = map[string]struct{}{
	"POST":   {},
	"GET":    {},
	"DELETE": {},
	"PUT":    {},
	"HEAD":   {},
	"PATCH":  {},
}

type JSONTarget struct {
	target        *ParsedURL
	method        string
	headers       map[string]string
	params        map[string]string
	payloadExtras map[string]string
}

func NewJSONTarget(target *ParsedURL) (*JSONTarget, error) {
	method := "POST"
	if rawMethod, ok := target.Query["method"]; ok && rawMethod != "" {
		method = strings.ToUpper(rawMethod)
	}
	if _, ok := jsonMethods[method]; !ok {
		return nil, fmt.Errorf("invalid method: %s", method)
	}

	return &JSONTarget{
		target:        target,
		method:        method,
		headers:       cloneMap(target.QueryAdd),
		params:        cloneMap(target.QueryDel),
		payloadExtras: cloneMap(target.QueryPayload),
	}, nil
}

func (j *JSONTarget) Send(body, title string, notifyType NotifyType) error {
	payload := map[string]any{
		"version":     "1.0",
		"title":       title,
		"message":     body,
		"attachments": []any{},
		"type":        string(notifyType),
	}

	for key, value := range j.payloadExtras {
		if existing, ok := payload[key]; ok {
			if value == "" {
				delete(payload, key)
			} else {
				payload[value] = existing
				delete(payload, key)
			}
			continue
		}
		payload[key] = value
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json encode: %w", err)
	}

	scheme := "http"
	if strings.ToLower(j.target.Scheme) == "jsons" {
		scheme = "https"
	}

	host := j.target.Host
	if j.target.Port != 0 {
		host = fmt.Sprintf("%s:%d", host, j.target.Port)
	}

	u := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   j.target.Path,
	}

	if len(j.params) > 0 {
		values := url.Values{}
		for key, value := range j.params {
			values.Set(key, value)
		}
		u.RawQuery = values.Encode()
	}

	req, err := http.NewRequest(j.method, u.String(), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Apprise")
	req.Header.Set("Content-Type", "application/json")
	for key, value := range j.headers {
		req.Header.Set(key, value)
	}

	if j.target.User != "" {
		req.SetBasicAuth(j.target.User, j.target.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func cloneMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
