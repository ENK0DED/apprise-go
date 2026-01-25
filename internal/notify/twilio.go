package notify

import (
	"fmt"
	"net/url"
	"strings"
)

const twilioSMSURLTemplate = "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json"
const twilioCallURLTemplate = "https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json"

type twilioMessageMode string

const (
	twilioModeText     twilioMessageMode = "text"
	twilioModeWhatsapp twilioMessageMode = "whatsapp"
)

const (
	twilioMethodSMS  = "sms"
	twilioMethodCall = "call"
)

type TwilioTarget struct {
	accountSID  string
	authToken   string
	apiKey      string
	method      string
	defaultMode twilioMessageMode
	source      string
	targets     []twilioTarget
}

type twilioTarget struct {
	mode   twilioMessageMode
	target string
}

func NewTwilioTarget(target *ParsedURL) (*TwilioTarget, error) {
	accountSID := strings.TrimSpace(target.User)
	authToken := target.Password
	if raw := strings.TrimSpace(target.Query["sid"]); raw != "" {
		accountSID = raw
	}
	if raw := strings.TrimSpace(target.Query["token"]); raw != "" {
		authToken = raw
	}
	if accountSID == "" || authToken == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	apiKey := strings.TrimSpace(target.Query["apikey"])

	method := twilioMethodSMS
	if raw := strings.TrimSpace(target.Query["method"]); raw != "" {
		normalized := strings.ToLower(raw)
		switch {
		case strings.HasPrefix(twilioMethodSMS, normalized):
			method = twilioMethodSMS
		case strings.HasPrefix(twilioMethodCall, normalized):
			method = twilioMethodCall
		default:
			return nil, fmt.Errorf("invalid method")
		}
	}

	sourceRaw := strings.TrimSpace(target.Host)
	if raw := strings.TrimSpace(target.Query["from"]); raw != "" {
		sourceRaw = raw
	} else if raw := strings.TrimSpace(target.Query["source"]); raw != "" {
		sourceRaw = raw
	}

	mode, _, sourceDigits, ok := parseTwilioModeAndNumber(sourceRaw)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}

	defaultMode := twilioModeText
	if mode == twilioModeWhatsapp {
		defaultMode = twilioModeWhatsapp
	}
	if method == twilioMethodCall && defaultMode == twilioModeWhatsapp {
		return nil, fmt.Errorf("invalid mode")
	}

	source, ok := formatTwilioSource(sourceDigits)
	if !ok {
		return nil, fmt.Errorf("invalid source")
	}

	rawTargets := splitPath(target.Path)
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		rawTargets = append(rawTargets, parseDelimitedList(toValue)...)
	}

	targets := []twilioTarget{}
	for _, entry := range rawTargets {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		mode, hasPrefix, targetDigits, ok := parseTwilioModeAndNumber(entry)
		if !ok {
			continue
		}
		if !hasPrefix {
			mode = defaultMode
		}
		if normalized, ok := normalizePhone(targetDigits); ok {
			if (len(sourceDigits) == 5 || len(sourceDigits) == 6 || method == twilioMethodCall) && mode == twilioModeWhatsapp {
				continue
			}
			targets = append(targets, twilioTarget{
				mode:   mode,
				target: "+" + normalized,
			})
		}
	}

	return &TwilioTarget{
		accountSID:  accountSID,
		authToken:   authToken,
		apiKey:      apiKey,
		method:      method,
		defaultMode: defaultMode,
		source:      source,
		targets:     targets,
	}, nil
}

func (t *TwilioTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	targets := t.targets
	sourceDigits := strings.TrimPrefix(t.source, "+")
	if len(targets) == 0 && (len(sourceDigits) == 5 || len(sourceDigits) == 6) {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}
	if len(targets) == 0 && t.method != twilioMethodCall {
		targets = []twilioTarget{{mode: t.defaultMode, target: t.source}}
	}
	if len(targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)

	payload := url.Values{}
	if t.method == twilioMethodCall {
		payload.Set("Twiml", message)
	} else {
		payload.Set("Body", message)
	}

	first := targets[0]
	if first.mode == twilioModeWhatsapp {
		payload.Set("From", "whatsapp:"+t.source)
		payload.Set("To", "whatsapp:"+first.target)
	} else {
		payload.Set("From", t.source)
		payload.Set("To", first.target)
	}

	authUser := t.accountSID
	if t.apiKey != "" {
		authUser = t.apiKey
	}

	requestURL := fmt.Sprintf(twilioSMSURLTemplate, t.accountSID)
	if t.method == twilioMethodCall {
		requestURL = fmt.Sprintf(twilioCallURLTemplate, t.accountSID)
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "application/json",
			"Content-Type": "application/x-www-form-urlencoded",
			"Authorization": basicAuthHeader(
				authUser,
				t.authToken,
			),
		},
		Body: payload.Encode(),
	}, nil
}

func (t *TwilioTarget) Send(body, title string, notifyType NotifyType) error {
	targets := t.targets
	sourceDigits := strings.TrimPrefix(t.source, "+")
	if len(targets) == 0 && (len(sourceDigits) == 5 || len(sourceDigits) == 6) {
		return fmt.Errorf("missing targets")
	}
	if len(targets) == 0 && t.method != twilioMethodCall {
		targets = []twilioTarget{{mode: t.defaultMode, target: t.source}}
	}
	if len(targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)

	authUser := t.accountSID
	if t.apiKey != "" {
		authUser = t.apiKey
	}

	requestURL := fmt.Sprintf(twilioSMSURLTemplate, t.accountSID)
	if t.method == twilioMethodCall {
		requestURL = fmt.Sprintf(twilioCallURLTemplate, t.accountSID)
	}

	for _, target := range targets {
		payload := url.Values{}
		if t.method == twilioMethodCall {
			payload.Set("Twiml", message)
		} else {
			payload.Set("Body", message)
		}

		if target.mode == twilioModeWhatsapp {
			payload.Set("From", "whatsapp:"+t.source)
			payload.Set("To", "whatsapp:"+target.target)
		} else {
			payload.Set("From", t.source)
			payload.Set("To", target.target)
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    requestURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Accept":       "application/json",
				"Content-Type": "application/x-www-form-urlencoded",
				"Authorization": basicAuthHeader(
					authUser,
					t.authToken,
				),
			},
			Body: payload.Encode(),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func parseTwilioModeAndNumber(raw string) (twilioMessageMode, bool, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, "", false
	}
	mode := twilioModeText
	phoneno := raw
	hasPrefix := false
	if colon := strings.Index(raw, ":"); colon >= 0 {
		left := strings.TrimSpace(raw[:colon])
		right := strings.TrimSpace(raw[colon+1:])
		if right == "" {
			return "", false, "", false
		}
		phoneno = right
		hasPrefix = true
		if left != "" && (left[0] == 'w' || left[0] == 'W') {
			mode = twilioModeWhatsapp
		} else {
			mode = twilioModeText
		}
	}

	digits, ok := normalizePhoneWithBounds(phoneno, 5, 14)
	if !ok {
		return "", false, "", false
	}

	return mode, hasPrefix, digits, true
}

func formatTwilioSource(digits string) (string, bool) {
	if len(digits) >= 11 && len(digits) <= 14 {
		return "+" + digits, true
	}
	if len(digits) == 5 || len(digits) == 6 {
		return digits, true
	}
	return "", false
}
