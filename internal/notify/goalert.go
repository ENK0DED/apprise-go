package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type GoAlertTarget struct {
	deliveryTracker
	endpoint, action, dedup string
	keys                    []string
	meta                    map[string]string
}

var goAlertKeyPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func NewGoAlertTarget(p *ParsedURL) (*GoAlertTarget, error) {
	action, err := incidentChoice(p.Query["action"], "map", "trigger", "close")
	if err != nil {
		return nil, err
	}
	t := &GoAlertTarget{action: action, dedup: strings.TrimSpace(p.Query["dedup"]), meta: p.QueryAdd}
	var path, keys []string
	for _, part := range splitPath(p.Path) {
		if goAlertKeyPattern.MatchString(part) {
			keys = append(keys, part)
		} else {
			path = append(path, part)
		}
	}
	for _, key := range []string{"to", "token"} {
		keys = append(keys, parseDelimitedList(p.Query[key])...)
	}
	for _, key := range sortedUniqueTargets(keys) {
		if goAlertKeyPattern.MatchString(key) {
			t.keys = append(t.keys, strings.ToLower(key))
		}
	}
	host := p.Host
	if p.HasPort {
		host += ":" + strconv.Itoa(p.Port)
	}
	scheme := "http"
	if p.Scheme == "goalerts" {
		scheme = "https"
	}
	prefix := ""
	if len(path) > 0 {
		prefix = pythonQuotePath(strings.Join(path, "/")) + "/"
	}
	t.endpoint = scheme + "://" + host + "/" + prefix + "api/v2/generic/incoming"
	return t, nil
}

func (t *GoAlertTarget) requestForKey(body, title string, kind NotifyType, key string) (RequestSpec, error) {
	if title == "" {
		// GoAlert's plain summary permits 1024 runes; the body retains Markdown.
		title = incidentPlainSummaryUnlimited(body)
		if runes := []rune(title); len(runes) > 1024 {
			title = string(runes[:1024])
		}
	}
	payload := map[string]any{"summary": title, "details": body}
	if t.action == "close" || (t.action == "map" && kind == NotifySuccess) {
		payload["action"] = "close"
	}
	if t.dedup != "" {
		payload["dedup"] = t.dedup
	}
	if len(t.meta) > 0 {
		payload["meta"] = t.meta
	}
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: t.endpoint, Headers: map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json", "Authorization": "Bearer " + key}, Body: string(data)}, err
}

func (t *GoAlertTarget) BuildRequest(body, title string, kind NotifyType) (RequestSpec, error) {
	if len(t.keys) == 0 {
		return RequestSpec{}, fmt.Errorf("no GoAlert integration keys")
	}
	return t.requestForKey(body, title, kind, t.keys[0])
}

func (t *GoAlertTarget) Send(body, title string, kind NotifyType) error {
	if len(t.keys) == 0 {
		return fmt.Errorf("no GoAlert integration keys")
	}
	var outcome sendOutcome
	for _, key := range t.keys {
		if t.isDelivered(key) {
			continue
		}
		spec, err := t.requestForKey(body, title, kind, key)
		if err == nil {
			err = sendGoAlertRequest(spec)
		}
		outcome.record(err)
		if err == nil {
			t.markDelivered(key)
		}
	}
	return outcome.err()
}

func sendGoAlertRequest(spec RequestSpec) error {
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case 200, 201, 202, 204:
		return nil
	default:
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
}
