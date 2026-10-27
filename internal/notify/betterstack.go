package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
)

type BetterStackTarget struct {
	deliveryTracker
	token, endpoint, from, policy string
	logs                          bool
	teams, alert                  []string
	teamWait                      *int
}

func NewBetterStackTarget(p *ParsedURL) (*BetterStackTarget, error) {
	t := &BetterStackTarget{token: p.Host, endpoint: "https://incidents.betterstack.com/api/v3/incidents", from: p.Query["from"], policy: strings.TrimSpace(p.Query["policy"])}
	if p.User != "" || p.Query["token"] != "" {
		t.logs = true
		t.token = p.User
		t.endpoint = "https://" + p.Host
		if p.HasPort {
			t.endpoint += ":" + strconv.Itoa(p.Port)
		}
		t.endpoint += "/"
	}
	if p.Query["token"] != "" {
		t.token = p.Query["token"]
	}
	if fields := strings.Fields(t.token); len(fields) > 0 {
		t.token = fields[0]
	} else {
		return nil, fmt.Errorf("missing Better Stack token")
	}
	for _, entry := range splitPath(p.Path) {
		if t.from == "" {
			if email, err := mail.ParseAddress(entry); err == nil {
				t.from = email.Address
				continue
			}
		}
		t.teams = append(t.teams, entry)
	}
	if t.from != "" {
		email, err := mail.ParseAddress(t.from)
		if err != nil {
			return nil, fmt.Errorf("invalid Better Stack requester")
		}
		t.from = email.Address
	}
	// Team names can contain spaces; only punctuation separates query teams.
	query := strings.NewReplacer(",", ";", "[", ";", "]", ";").Replace(p.Query["to"])
	t.teams = sortedUniqueTargets(append(t.teams, strings.Split(query, ";")...))
	if p.Query["alert"] != "" {
		t.alert = sortedUniqueTargets(parseDelimitedList(strings.ToLower(p.Query["alert"])))
		for _, method := range t.alert {
			if _, ok := betterStackMethods[method]; !ok {
				return nil, fmt.Errorf("invalid Better Stack alert method")
			}
		}
	}
	if value := p.Query["team_wait"]; value != "" {
		wait, err := strconv.Atoi(value)
		if err != nil || wait < 0 {
			return nil, fmt.Errorf("invalid Better Stack team wait")
		}
		t.teamWait = &wait
	}
	return t, nil
}

var betterStackMethods = map[string]string{"call": "call", "sms": "sms", "email": "email", "push": "push", "critical": "critical_alert"}

func (t *BetterStackTarget) requestForTeam(body, title string, kind NotifyType, team string) (RequestSpec, error) {
	var payload map[string]any
	if t.logs {
		level := "info"
		if kind == NotifyWarning {
			level = "warn"
		}
		if kind == NotifyFailure {
			level = "error"
		}
		payload = map[string]any{"dt": fixedTime().UTC().Format("2006-01-02T15:04:05.999999999-07:00"), "level": level, "message": body}
		if title != "" {
			payload["title"] = title
		}
	} else {
		summary := title
		if summary == "" {
			summary = body
		}
		payload = map[string]any{"summary": summary}
		if title != "" {
			payload["name"] = title
			payload["description"] = body
		}
		if t.from != "" {
			payload["requester_email"] = t.from
		}
		if t.policy != "" {
			payload["policy_id"] = t.policy
		}
		if t.teamWait != nil {
			payload["team_wait"] = *t.teamWait
		}
		if t.alert != nil {
			for method, field := range betterStackMethods {
				enabled := false
				for _, value := range t.alert {
					if method == value {
						enabled = true
					}
				}
				payload[field] = enabled
			}
		}
		if team != "" {
			payload["team_name"] = team
		}
	}
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: t.endpoint, Headers: map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json", "Authorization": "Bearer " + t.token}, Body: string(data)}, err
}

func (t *BetterStackTarget) BuildRequest(body, title string, kind NotifyType) (RequestSpec, error) {
	team := ""
	if len(t.teams) > 0 {
		team = t.teams[0]
	}
	return t.requestForTeam(body, title, kind, team)
}

func (t *BetterStackTarget) Send(body, title string, kind NotifyType) error {
	teams := t.teams
	if t.logs || len(teams) == 0 {
		teams = []string{""}
	}
	var outcome sendOutcome
	for _, team := range teams {
		if t.isDelivered(team) {
			continue
		}
		spec, err := t.requestForTeam(body, title, kind, team)
		if err == nil {
			err = sendGoAlertRequest(spec)
		}
		outcome.record(err)
		if err == nil {
			t.markDelivered(team)
		}
	}
	return outcome.err()
}
