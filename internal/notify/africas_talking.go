package notify

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	africasTalkingBulkURL    = "https://api.africastalking.com/version1/messaging"
	africasTalkingPremiumURL = "https://content.africastalking.com/version1/messaging"
	africasTalkingSandboxURL = "https://api.sandbox.africastalking.com/version1/messaging"

	africasTalkingDefaultSender   = "AFRICASTKNG"
	africasTalkingDefaultMode     = "bulksms"
	africasTalkingDefaultBatchLen = 50
)

var africasTalkingTokenRe = regexp.MustCompile(`(?i)^[a-z0-9_-]+$`)
var africasTalkingModes = []string{"bulksms", "premium", "sandbox"}

type AfricasTalkingTarget struct {
	appuser string
	apikey  string
	sender  string
	mode    string
	batch   bool
	targets []string
}

func NewAfricasTalkingTarget(target *ParsedURL) (*AfricasTalkingTarget, error) {
	appuser := strings.TrimSpace(target.User)
	if rawUser := strings.TrimSpace(target.Query["user"]); rawUser != "" {
		appuser = rawUser
	}
	if appuser == "" {
		return nil, fmt.Errorf("missing appuser")
	}
	if !africasTalkingTokenRe.MatchString(appuser) {
		return nil, fmt.Errorf("invalid appuser")
	}

	apikey := strings.TrimSpace(target.Host)
	hostTarget := ""
	if rawKey := strings.TrimSpace(target.Query["apikey"]); rawKey != "" {
		apikey = rawKey
		hostTarget = strings.TrimSpace(target.Host)
	} else if rawKey := strings.TrimSpace(target.Query["key"]); rawKey != "" {
		apikey = rawKey
		hostTarget = strings.TrimSpace(target.Host)
	}
	if apikey == "" {
		return nil, fmt.Errorf("missing apikey")
	}
	if !africasTalkingTokenRe.MatchString(apikey) {
		return nil, fmt.Errorf("invalid apikey")
	}

	sender := strings.TrimSpace(target.Query["from"])
	if rawSender := strings.TrimSpace(target.Query["sender"]); rawSender != "" {
		sender = rawSender
	}
	if sender == "" {
		sender = africasTalkingDefaultSender
	}

	mode := africasTalkingDefaultMode
	if rawMode := strings.TrimSpace(target.Query["mode"]); rawMode != "" {
		normalized, ok := normalizeAfricasTalkingMode(rawMode)
		if !ok {
			return nil, fmt.Errorf("invalid mode")
		}
		mode = normalized
	}

	batch := parseBoolWithDefault(target.Query["batch"], false)

	entries := []string{}
	if hostTarget != "" {
		entries = append(entries, hostTarget)
	}
	entries = append(entries, splitPath(target.Path)...)
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}

	targets := []string{}
	for _, entry := range entries {
		normalized, ok := normalizePhoneWithPlus(entry)
		if !ok {
			continue
		}
		targets = append(targets, normalized)
	}

	return &AfricasTalkingTarget{
		appuser: appuser,
		apikey:  apikey,
		sender:  sender,
		mode:    mode,
		batch:   batch,
		targets: targets,
	}, nil
}

func (a *AfricasTalkingTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(a.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if a.batch {
		batchSize = africasTalkingDefaultBatchLen
	}
	chunk := a.targets
	if len(chunk) > batchSize {
		chunk = chunk[:batchSize]
	}

	spec, err := a.buildRequest(chunk, message)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	return spec, nil
}

func (a *AfricasTalkingTarget) Send(body, title string, notifyType NotifyType) error {
	if len(a.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	batchSize := 1
	if a.batch {
		batchSize = africasTalkingDefaultBatchLen
	}

	for idx := 0; idx < len(a.targets); idx += batchSize {
		end := idx + batchSize
		if end > len(a.targets) {
			end = len(a.targets)
		}
		spec, err := a.buildRequest(a.targets[idx:end], message)
		if err != nil {
			return err
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (a *AfricasTalkingTarget) buildRequest(targets []string, message string) (RequestSpec, error) {
	requestURL, ok := africasTalkingModeURL(a.mode)
	if !ok {
		return RequestSpec{}, fmt.Errorf("invalid mode")
	}

	payload := url.Values{}
	payload.Set("username", a.appuser)
	payload.Set("to", strings.Join(targets, ","))
	payload.Set("from", a.sender)
	payload.Set("message", message)

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded",
			"Accept":       "application/json",
			"apiKey":       a.apikey,
		},
		Body: payload.Encode(),
	}, nil
}

func normalizeAfricasTalkingMode(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return africasTalkingDefaultMode, true
	}
	for _, mode := range africasTalkingModes {
		if strings.HasPrefix(mode, raw) {
			return mode, true
		}
	}
	return "", false
}

func africasTalkingModeURL(mode string) (string, bool) {
	switch mode {
	case "bulksms":
		return africasTalkingBulkURL, true
	case "premium":
		return africasTalkingPremiumURL, true
	case "sandbox":
		return africasTalkingSandboxURL, true
	default:
		return "", false
	}
}
