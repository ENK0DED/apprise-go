package notify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	sendPulseEmailURL = "https://api.sendpulse.com/smtp/emails"
	sendPulseOAuthURL = "https://api.sendpulse.com/oauth/access_token"
	sendPulseSubject  = "<no subject>"
)

type SendPulseTarget struct {
	clientID     string
	clientSecret string
	fromAddr     string
	fromName     string
	targets      []string
	cc           map[string]struct{}
	bcc          map[string]struct{}
	templateID   int
	templateData map[string]string
}

func NewSendPulseTarget(target *ParsedURL) (*SendPulseTarget, error) {
	entries := splitPath(target.Path)
	clientID := strings.TrimSpace(target.Query["id"])
	if clientID == "" && len(entries) > 0 {
		clientID = strings.TrimSpace(entries[0])
		entries = entries[1:]
	}
	if clientID == "" {
		return nil, fmt.Errorf("missing client id")
	}

	clientSecret := strings.TrimSpace(target.Query["secret"])
	if clientSecret == "" && len(entries) > 0 {
		clientSecret = strings.TrimSpace(entries[0])
		entries = entries[1:]
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("missing client secret")
	}

	user := strings.TrimSpace(target.User)
	host := strings.TrimSpace(target.Host)
	fromAddr := ""
	if rawFrom := strings.TrimSpace(target.Query["from"]); rawFrom != "" {
		if isSimpleEmail(rawFrom) {
			fromAddr = rawFrom
		}
	} else if user != "" && host != "" {
		userPart := strings.FieldsFunc(user, func(r rune) bool {
			return r == '@' || r == ' ' || r == '\t' || r == '\n'
		})
		if len(userPart) > 0 {
			fromAddr = userPart[0] + "@" + host
		}
	} else if user != "" && isSimpleEmail(user) {
		fromAddr = user
	}

	if !isSimpleEmail(fromAddr) {
		return nil, fmt.Errorf("invalid from address")
	}

	targets := []string{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if isSimpleEmail(entry) {
			targets = append(targets, entry)
		}
	}
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				targets = append(targets, entry)
			}
		}
	}
	if len(targets) == 0 {
		targets = append(targets, fromAddr)
	}

	cc := map[string]struct{}{}
	if ccValue := strings.TrimSpace(target.Query["cc"]); ccValue != "" {
		for _, entry := range parseDelimitedList(ccValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				cc[entry] = struct{}{}
			}
		}
	}

	bcc := map[string]struct{}{}
	if bccValue := strings.TrimSpace(target.Query["bcc"]); bccValue != "" {
		for _, entry := range parseDelimitedList(bccValue) {
			entry = strings.TrimSpace(entry)
			if isSimpleEmail(entry) {
				bcc[entry] = struct{}{}
			}
		}
	}

	templateID := 0
	if templateValue := strings.TrimSpace(target.Query["template"]); templateValue != "" {
		if parsed, err := strconv.Atoi(templateValue); err == nil {
			templateID = parsed
		} else {
			return nil, fmt.Errorf("invalid template id")
		}
	}

	templateData := map[string]string{}
	for key, value := range target.QueryAdd {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		templateData[key] = value
	}

	return &SendPulseTarget{
		clientID:     clientID,
		clientSecret: clientSecret,
		fromAddr:     fromAddr,
		fromName:     "Apprise",
		targets:      targets,
		cc:           cc,
		bcc:          bcc,
		templateID:   templateID,
		templateData: templateData,
	}, nil
}

func (s *SendPulseTarget) Send(body, title string, notifyType NotifyType) error {
	token, err := s.login()
	if err != nil {
		return err
	}

	for _, target := range s.targets {
		payload := s.buildEmailPayload(body, title, target)
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", sendPulseEmailURL, strings.NewReader(string(data)))
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Apprise")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "*/*")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return &HTTPStatusError{StatusCode: resp.StatusCode}
		}
	}

	_ = notifyType
	return nil
}

func (s *SendPulseTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     s.clientID,
		"client_secret": s.clientSecret,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = body
	_ = title
	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    sendPulseOAuthURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (s *SendPulseTarget) login() (string, error) {
	spec, err := s.BuildRequest("", "", NotifyInfo)
	if err != nil {
		return "", err
	}

	req, err := spec.HTTPRequest()
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	var response struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	if response.AccessToken == "" {
		return "", fmt.Errorf("missing access token")
	}
	return response.AccessToken, nil
}

func (s *SendPulseTarget) buildEmailPayload(body, title, target string) map[string]any {
	subject := title
	if subject == "" {
		subject = sendPulseSubject
	}

	emailPayload := map[string]any{
		"from": map[string]any{
			"name":  s.fromName,
			"email": s.fromAddr,
		},
		"to": []map[string]any{{
			"email": target,
		}},
		"subject": subject,
		"text":    body,
		"html":    base64.StdEncoding.EncodeToString([]byte(body)),
	}

	if len(s.cc) > 0 {
		ccList := []map[string]any{}
		for entry := range s.cc {
			if entry == target {
				continue
			}
			ccList = append(ccList, map[string]any{
				"email": entry,
			})
		}
		if len(ccList) > 0 {
			emailPayload["cc"] = ccList
		}
	}

	if len(s.bcc) > 0 {
		bccList := []map[string]any{}
		for entry := range s.bcc {
			if entry == target {
				continue
			}
			bccList = append(bccList, map[string]any{
				"email": entry,
			})
		}
		if len(bccList) > 0 {
			emailPayload["bcc"] = bccList
		}
	}

	if s.templateID > 0 {
		emailPayload["template"] = map[string]any{
			"id":        s.templateID,
			"variables": s.templateData,
		}
	}

	return map[string]any{
		"email": emailPayload,
	}
}
