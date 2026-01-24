package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var formMethods = map[string]struct{}{
	"POST":   {},
	"GET":    {},
	"DELETE": {},
	"PUT":    {},
	"HEAD":   {},
	"PATCH":  {},
}

type FormTarget struct {
	target        *ParsedURL
	method        string
	headers       map[string]string
	params        map[string]string
	payloadExtras map[string]string
	payloadMap    map[string]string
}

func NewFormTarget(target *ParsedURL) (*FormTarget, error) {
	method := "POST"
	if rawMethod, ok := target.Query["method"]; ok && rawMethod != "" {
		method = strings.ToUpper(rawMethod)
	}
	if _, ok := formMethods[method]; !ok {
		return nil, fmt.Errorf("invalid method: %s", method)
	}

	payloadExtras := cloneMap(target.QueryPayload)
	payloadMap := map[string]string{
		"version": "version",
		"title":   "title",
		"message": "message",
		"type":    "type",
	}

	for key, value := range payloadExtras {
		if _, ok := payloadMap[key]; !ok {
			continue
		}

		payloadMap[key] = value
		delete(payloadExtras, key)
	}

	return &FormTarget{
		target:        target,
		method:        method,
		headers:       cloneMap(target.QueryAdd),
		params:        cloneMap(target.QueryDel),
		payloadExtras: payloadExtras,
		payloadMap:    payloadMap,
	}, nil
}

func (f *FormTarget) Send(body, title string, notifyType NotifyType) error {
	payload := map[string]string{}

	base := map[string]string{
		"version": "1.0",
		"title":   title,
		"message": body,
		"type":    string(notifyType),
	}

	for key, value := range base {
		mapped := f.payloadMap[key]
		if mapped == "" {
			continue
		}
		payload[mapped] = value
	}

	for key, value := range f.payloadExtras {
		payload[key] = value
	}

	scheme := "http"
	if strings.ToLower(f.target.Scheme) == "forms" {
		scheme = "https"
	}

	host := f.target.Host
	if f.target.Port != 0 {
		host = fmt.Sprintf("%s:%d", host, f.target.Port)
	}

	u := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   f.target.Path,
	}

	if f.method == "GET" {
		values := url.Values{}
		for key, value := range payload {
			values.Set(key, value)
		}
		for key, value := range f.params {
			values.Set(key, value)
		}
		u.RawQuery = values.Encode()
	}

	var bodyReader *strings.Reader
	if f.method != "GET" {
		values := url.Values{}
		for key, value := range payload {
			values.Set(key, value)
		}
		bodyReader = strings.NewReader(values.Encode())
	}

	req, err := http.NewRequest(f.method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if f.method != "GET" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if f.method != "GET" && len(f.params) > 0 {
		values := url.Values{}
		for key, value := range f.params {
			values.Set(key, value)
		}
		req.URL.RawQuery = values.Encode()
	}

	req.Header.Set("User-Agent", "Apprise")
	for key, value := range f.headers {
		req.Header.Set(key, value)
	}

	if f.target.User != "" {
		req.SetBasicAuth(f.target.User, f.target.Password)
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
