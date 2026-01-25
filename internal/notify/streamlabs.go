package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const streamlabsBaseURL = "https://streamlabs.com/api/v1.0/"
const streamlabsCallAlerts = "ALERTS"
const streamlabsCallDonations = "DONATIONS"

var streamlabsAlertTypes = map[string]struct{}{
	"follow":       {},
	"subscription": {},
	"donation":     {},
	"host":         {},
}

type StreamlabsTarget struct {
	accessToken      string
	call             string
	alertType        string
	imageHref        string
	soundHref        string
	duration         int
	specialTextColor string
	amount           int
	currency         string
	name             string
	identifier       string
}

func NewStreamlabsTarget(target *ParsedURL) (*StreamlabsTarget, error) {
	accessToken := strings.TrimSpace(target.Host)
	if accessToken == "" {
		return nil, fmt.Errorf("missing access token")
	}

	call := normalizeStreamlabsCall(target.Query["call"])
	alertType := normalizeStreamlabsAlertType(target.Query["alert_type"])

	duration := parseIntWithDefault(target.Query["duration"], 1000)
	amount := parseIntWithDefault(target.Query["amount"], 0)

	currency := strings.ToUpper(strings.TrimSpace(target.Query["currency"]))
	if currency == "" {
		currency = "USD"
	}
	if len(currency) != 3 {
		currency = "USD"
	}

	name := strings.ToUpper(strings.TrimSpace(target.Query["name"]))
	if name == "" {
		name = "Anon"
	}

	identifier := strings.ToUpper(strings.TrimSpace(target.Query["identifier"]))
	if identifier == "" {
		identifier = "Apprise"
	}

	return &StreamlabsTarget{
		accessToken:      accessToken,
		call:             call,
		alertType:        alertType,
		imageHref:        target.Query["image_href"],
		soundHref:        strings.ToUpper(strings.TrimSpace(target.Query["sound_href"])),
		duration:         duration,
		specialTextColor: strings.ToUpper(strings.TrimSpace(target.Query["special_text_color"])),
		amount:           amount,
		currency:         currency,
		name:             name,
		identifier:       identifier,
	}, nil
}

func (s *StreamlabsTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	values := url.Values{}
	requestURL := streamlabsBaseURL + strings.ToLower(s.call)

	if s.call == streamlabsCallDonations {
		values.Set("name", s.name)
		values.Set("identifier", s.identifier)
		values.Set("amount", strconv.Itoa(s.amount))
		values.Set("currency", s.currency)
		values.Set("access_token", s.accessToken)
		values.Set("message", body)
	} else {
		values.Set("access_token", s.accessToken)
		values.Set("type", s.alertType)
		values.Set("image_href", s.imageHref)
		values.Set("sound_href", s.soundHref)
		values.Set("message", title)
		values.Set("user_massage", body)
		values.Set("duration", strconv.Itoa(s.duration))
		values.Set("special_text_color", s.specialTextColor)
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: values.Encode(),
	}, nil
}

func (s *StreamlabsTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func normalizeStreamlabsCall(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "ALERT", "ALERTS":
		return streamlabsCallAlerts
	case "DONATION", "DONATIONS":
		return streamlabsCallDonations
	default:
		return streamlabsCallAlerts
	}
}

func normalizeStreamlabsAlertType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := streamlabsAlertTypes[value]; ok {
		return value
	}
	return "donation"
}

func parseIntWithDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
