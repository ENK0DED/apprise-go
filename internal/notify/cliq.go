package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type CliqTarget struct {
	deliveryTracker
	parsed                          *ParsedURL
	token, domain, company, botname string
	image, broadcast                bool
	targets                         []string
}

var cliqTokenPattern = regexp.MustCompile(`(?i)^[a-z0-9]+(\.[a-z0-9]+)+$`)
var cliqNamePattern = regexp.MustCompile(`(?i)^[@#]?[a-z0-9_.-]+$`)
var cliqCompanyPattern = regexp.MustCompile(`(?i)^[a-z0-9_-]+$`)
var cliqDomains = map[string]string{"us": "cliq.zoho.com", "eu": "cliq.zoho.eu", "in": "cliq.zoho.in", "au": "cliq.zoho.com.au", "cn": "cliq.zoho.com.cn", "jp": "cliq.zoho.jp", "sa": "cliq.zoho.sa", "uk": "cliq.zoho.uk", "ca": "cliq.zohocloud.ca"}

func NewCliqTarget(p *ParsedURL) (*CliqTarget, error) {
	t := &CliqTarget{parsed: p, token: p.Host, company: p.Query["company"], botname: strings.TrimSpace(p.User), image: parseBoolWithDefault(p.Query["image"], true), broadcast: parseBoolWithDefault(p.Query["broadcast"], false)}
	if p.Query["token"] != "" {
		t.token = p.Query["token"]
	}
	if !cliqTokenPattern.MatchString(t.token) {
		return nil, fmt.Errorf("invalid Zoho Cliq token")
	}
	region := strings.ToLower(strings.TrimSpace(p.Query["region"]))
	if region == "" {
		region = "us"
	}
	t.domain = cliqDomains[region]
	if t.domain == "" {
		return nil, fmt.Errorf("invalid Zoho Cliq region")
	}
	if t.company != "" && !cliqCompanyPattern.MatchString(t.company) {
		return nil, fmt.Errorf("invalid Zoho Cliq company")
	}
	if p.Query["botname"] != "" {
		t.botname = strings.TrimSpace(p.Query["botname"])
	}
	entries := sortedUniqueTargets(append(splitPath(p.Path), parseDelimitedList(p.Query["to"])...))
	for _, entry := range entries {
		if !cliqNamePattern.MatchString(entry) {
			continue
		}
		if entry[0] != '@' && entry[0] != '#' {
			entry = "#" + entry
		}
		t.targets = append(t.targets, entry)
	}
	return t, nil
}

func (t *CliqTarget) request(body, title string, kind NotifyType, target string) (RequestSpec, error) {
	if !t.parsed.dialectApplied && t.parsed.sourceNotifyFormat != "" && targetNotifyFormat(t.parsed) == "markdown" {
		body = commonMarkToCliq(body)
	}
	text := body
	if text == "" {
		text = title
	}
	payload := map[string]any{"text": text}
	if title != "" && body != "" {
		payload["card"] = map[string]string{"title": title, "theme": "modern-inline"}
	}
	if t.image || t.botname != "" {
		name := t.botname
		if name == "" {
			name = "Apprise"
		}
		bot := map[string]string{"name": name}
		if t.image {
			bot["image"] = appriseImageURL(kind, "72x72")
		}
		payload["bot"] = bot
	}
	path := "channelsbyname"
	if target[0] == '@' {
		path = "bots"
		if t.broadcast {
			payload["broadcast"] = true
		}
	}
	company := ""
	if t.company != "" {
		company = "/company/" + t.company
	}
	endpoint := "https://" + t.domain + company + "/api/v2/" + path + "/" + url.PathEscape(target[1:]) + "/message?" + url.Values{"zapikey": []string{t.token}}.Encode()
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: endpoint, Headers: map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json; charset=utf-8"}, Body: string(data)}, err
}

func (t *CliqTarget) BuildRequest(body, title string, kind NotifyType) (RequestSpec, error) {
	if len(t.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("no Zoho Cliq recipients")
	}
	return t.request(body, title, kind, t.targets[0])
}
func (t *CliqTarget) Send(body, title string, kind NotifyType) error {
	if len(t.targets) == 0 {
		return fmt.Errorf("no Zoho Cliq recipients")
	}
	var outcome sendOutcome
	for _, target := range t.targets {
		if t.isDelivered(target) {
			continue
		}
		spec, err := t.request(body, title, kind, target)
		if err == nil {
			err = sendCliqRequest(spec)
		}
		outcome.record(err)
		if err == nil {
			t.markDelivered(target)
		}
	}
	return outcome.err()
}
func sendCliqRequest(spec RequestSpec) error {
	req, err := spec.HTTPRequest()
	if err != nil {
		return err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 && response.StatusCode != 204 {
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	return nil
}
