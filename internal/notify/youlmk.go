package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type YouLMKTarget struct {
	token, priority, link, label, group string
	priorities                          map[NotifyType]string
	image                               bool
}

var youlmkTokenPattern = regexp.MustCompile(`(?i)^(ylk_[a-z0-9]{32}|k_[a-z0-9]{14})$`)

func NewYouLMKTarget(p *ParsedURL) (*YouLMKTarget, error) {
	t := &YouLMKTarget{token: p.Host, link: p.Query["url"], label: p.Query["url_label"], group: p.Query["group"], image: parseBoolWithDefault(p.Query["image"], false), priorities: map[NotifyType]string{NotifyInfo: "normal", NotifySuccess: "normal", NotifyWarning: "high", NotifyFailure: "high"}}
	if p.Query["token"] != "" {
		t.token = p.Query["token"]
	}
	if !youlmkTokenPattern.MatchString(t.token) {
		return nil, fmt.Errorf("invalid YouLMK token")
	}
	for _, key := range []string{"priority", "info", "success", "warning", "failure"} {
		if p.Query[key] == "" {
			continue
		}
		value, err := incidentChoice(p.Query[key], "low", "normal", "high", "critical")
		if err != nil {
			return nil, err
		}
		if key == "priority" {
			t.priority = value
		} else {
			t.priorities[NotifyType(key)] = value
		}
	}
	if runes := []rune(t.label); len(runes) > 24 {
		t.label = string(runes[:24])
	}
	return t, nil
}

func (t *YouLMKTarget) BuildRequest(body, title string, kind NotifyType) (RequestSpec, error) {
	if title == "" {
		title = appriseAppDesc
	}
	priority := t.priority
	if priority == "" {
		priority = t.priorities[kind]
	}
	payload := map[string]string{"title": title, "body": body, "priority": priority}
	if t.link != "" {
		payload["url"] = t.link
		if t.label != "" {
			payload["url_label"] = t.label
		}
	}
	if t.group != "" {
		payload["group"] = t.group
	}
	if t.image {
		payload["image"] = appriseImageURL(kind, "256x256")
	}
	headers := map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json; charset=utf-8"}
	endpoint := "https://youlmk.com/k/" + t.token
	if strings.HasPrefix(t.token, "ylk_") {
		endpoint = "https://youlmk.com/v1/notify"
		headers["Authorization"] = "Bearer " + t.token
	}
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: endpoint, Headers: headers, Body: string(data)}, err
}

func (t *YouLMKTarget) Send(body, title string, kind NotifyType) error {
	spec, err := t.BuildRequest(body, title, kind)
	if err != nil {
		return err
	}
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	return nil
}
