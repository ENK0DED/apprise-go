package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

type LFGAlarmTarget struct{ token string }

var lfgTokenPattern = regexp.MustCompile(`(?i)^[a-z0-9]{32}$`)

func NewLFGAlarmTarget(p *ParsedURL) (*LFGAlarmTarget, error) {
	token := p.Host
	if p.Query["token"] != "" {
		token = p.Query["token"]
	}
	if !lfgTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("invalid LFG Alarm token")
	}
	return &LFGAlarmTarget{token: token}, nil
}

func (t *LFGAlarmTarget) BuildRequest(_, _ string, _ NotifyType) (RequestSpec, error) {
	return RequestSpec{Method: http.MethodPost, URL: "https://api.lfg.gm/horn/" + t.token, Headers: map[string]string{"User-Agent": "Apprise"}}, nil
}

func (t *LFGAlarmTarget) Send(body, title string, kind NotifyType) error {
	spec, _ := t.BuildRequest(body, title, kind)
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
	var result map[string]any
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("invalid LFG Alarm response: %w", err)
	}
	if result["fired"] != true {
		return fmt.Errorf("LFG Alarm did not ring")
	}
	return nil
}
