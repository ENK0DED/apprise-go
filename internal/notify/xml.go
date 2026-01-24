package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var xmlMethods = map[string]struct{}{
	"POST":   {},
	"GET":    {},
	"DELETE": {},
	"PUT":    {},
	"HEAD":   {},
	"PATCH":  {},
}

const (
	xmlVersion    = "1.1"
	xmlDefaultURL = "https://raw.githubusercontent.com/caronc/apprise/master/apprise/assets/NotifyXML-1.1.xsd"
)

const xmlTemplate = "" +
	"<?xml version='1.0' encoding='utf-8'?>\n" +
	"<soapenv:Envelope\n" +
	"    xmlns:soapenv=\"http://schemas.xmlsoap.org/soap/envelope/\"\n" +
	"    xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\"\n" +
	"    xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\">\n" +
	"    <soapenv:Body>\n" +
	"        <Notification{{XSD_URL}}>\n" +
	"            {{CORE}}\n" +
	"            {{ATTACHMENTS}}\n" +
	"       </Notification>\n" +
	"    </soapenv:Body>\n" +
	"</soapenv:Envelope>"

var xmlKeySanitizer = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

type XMLTarget struct {
	target        *ParsedURL
	method        string
	headers       map[string]string
	payloadExtras map[string]string
	payloadMap    map[string]string
	xsdURL        string
}

func NewXMLTarget(target *ParsedURL) (*XMLTarget, error) {
	method := "POST"
	if rawMethod, ok := target.Query["method"]; ok && rawMethod != "" {
		method = strings.ToUpper(rawMethod)
	}
	if _, ok := xmlMethods[method]; !ok {
		return nil, fmt.Errorf("invalid method: %s", method)
	}

	payloadExtras := map[string]string{}
	payloadMap := map[string]string{
		"Version":     "Version",
		"Subject":     "Subject",
		"Message":     "Message",
		"MessageType": "MessageType",
	}

	payloadOverrides := false
	for key, value := range target.QueryPayload {
		sanitized := sanitizeXMLKey(key)
		if sanitized == "" {
			continue
		}
		if _, ok := payloadMap[sanitized]; ok {
			payloadMap[sanitized] = value
			payloadOverrides = true
			continue
		}
		payloadExtras[sanitized] = value
	}

	xsdURL := xmlDefaultURL
	if payloadOverrides || len(payloadExtras) > 0 {
		xsdURL = ""
	}

	return &XMLTarget{
		target:        target,
		method:        method,
		headers:       cloneMap(target.QueryAdd),
		payloadExtras: payloadExtras,
		payloadMap:    payloadMap,
		xsdURL:        xsdURL,
	}, nil
}

func (x *XMLTarget) Send(body, title string, notifyType NotifyType) error {
	payloadBase := []struct {
		key   string
		value string
	}{
		{"Version", xmlVersion},
		{"Subject", escapeXML(title)},
		{"Message", escapeXML(body)},
		{"MessageType", escapeXML(string(notifyType))},
	}

	entries := []string{}
	for _, entry := range payloadBase {
		mapped := x.payloadMap[entry.key]
		if mapped == "" {
			continue
		}
		entries = append(entries, fmt.Sprintf("<%s>%s</%s>", mapped, entry.value, mapped))
	}

	for key, value := range x.payloadExtras {
		entries = append(entries, fmt.Sprintf("<%s>%s</%s>", key, escapeXML(value), key))
	}

	xsdAttr := ""
	if x.xsdURL != "" {
		xsdAttr = fmt.Sprintf(" xmlns:xsi=\"%s\"", x.xsdURL)
	}

	payload := strings.ReplaceAll(xmlTemplate, "{{XSD_URL}}", xsdAttr)
	payload = strings.ReplaceAll(payload, "{{CORE}}", strings.Join(entries, ""))
	payload = strings.ReplaceAll(payload, "{{ATTACHMENTS}}", "")

	scheme := "http"
	if strings.ToLower(x.target.Scheme) == "xmls" {
		scheme = "https"
	}

	host := x.target.Host
	if x.target.Port != 0 {
		host = fmt.Sprintf("%s:%d", host, x.target.Port)
	}

	u := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   x.target.Path,
	}

	req, err := http.NewRequest(x.method, u.String(), strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Apprise")
	req.Header.Set("Content-Type", "application/xml")
	for key, value := range x.headers {
		req.Header.Set(key, value)
	}

	if x.target.User != "" {
		req.SetBasicAuth(x.target.User, x.target.Password)
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

func sanitizeXMLKey(value string) string {
	return xmlKeySanitizer.ReplaceAllString(value, "")
}

func escapeXML(value string) string {
	if value == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
