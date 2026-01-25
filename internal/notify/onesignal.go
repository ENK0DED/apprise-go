package notify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const onesignalURL = "https://api.onesignal.com/notifications"
const onesignalBatchSize = 2000

const (
	oneSignalCategoryPlayer  = "include_player_ids"
	oneSignalCategoryEmail   = "include_email_tokens"
	oneSignalCategoryUser    = "include_external_user_ids"
	oneSignalCategorySegment = "included_segments"
)

var oneSignalCategoryOrder = []string{
	oneSignalCategoryPlayer,
	oneSignalCategoryEmail,
	oneSignalCategoryUser,
	oneSignalCategorySegment,
}

var oneSignalEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type OneSignalTarget struct {
	appID         string
	apiKey        string
	templateID    string
	includeImage  bool
	batchSize     int
	useContents   bool
	decodeTplArgs bool
	subtitle      string
	language      string
	targets       map[string][]string
	customData    map[string]string
	postbackData  map[string]string
}

func NewOneSignalTarget(target *ParsedURL) (*OneSignalTarget, error) {
	appID := strings.TrimSpace(target.User)
	templateID := ""
	if target.Password != "" {
		templateID = appID
		appID = target.Password
	}

	if rawApp := strings.TrimSpace(target.Query["app"]); rawApp != "" {
		appID = rawApp
	}
	if rawTemplate := strings.TrimSpace(target.Query["template"]); rawTemplate != "" {
		templateID = rawTemplate
	}

	apiKey := strings.TrimSpace(target.Host)
	if rawAPI := strings.TrimSpace(target.Query["apikey"]); rawAPI != "" {
		apiKey = rawAPI
	}

	if appID == "" || apiKey == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	includeImage := parseBoolWithDefault(target.Query["image"], true)

	batch := parseBoolWithDefault(target.Query["batch"], false)
	batchSize := 1
	if batch {
		batchSize = onesignalBatchSize
	}

	useContents := parseBoolWithDefault(target.Query["contents"], true)
	decodeTplArgs := parseBoolWithDefault(target.Query["decode"], false)

	subtitle := strings.TrimSpace(target.Query["subtitle"])

	language := strings.TrimSpace(target.Query["lang"])
	if language == "" {
		language = strings.TrimSpace(target.Query["language"])
	}
	if language == "" {
		language = "en"
	}
	language = strings.ToLower(language)
	if len(language) > 2 {
		language = language[:2]
	}
	if len(language) != 2 {
		return nil, fmt.Errorf("invalid language")
	}

	entries := []string{}
	entries = append(entries, splitPath(target.Path)...)
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		entries = append(entries, parseDelimitedList(toValue)...)
	}
	entries = normalizeOneSignalTargets(entries)

	targets := map[string][]string{
		oneSignalCategoryPlayer:  {},
		oneSignalCategoryEmail:   {},
		oneSignalCategoryUser:    {},
		oneSignalCategorySegment: {},
	}

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if len(entry) < 2 {
			continue
		}
		if strings.HasPrefix(entry, "@") {
			targets[oneSignalCategoryUser] = append(targets[oneSignalCategoryUser], entry)
			continue
		}
		if strings.HasPrefix(entry, "#") {
			targets[oneSignalCategorySegment] = append(targets[oneSignalCategorySegment], entry)
			continue
		}
		if oneSignalEmailRe.MatchString(entry) {
			targets[oneSignalCategoryEmail] = append(targets[oneSignalCategoryEmail], entry)
			continue
		}
		targets[oneSignalCategoryPlayer] = append(targets[oneSignalCategoryPlayer], entry)
	}

	customData := map[string]string{}
	for key, value := range target.QueryPayload {
		customData[key] = value
	}
	if decodeTplArgs && len(customData) > 0 {
		customData = decodeBase64Map(customData)
	}

	postbackData := map[string]string{}
	for key, value := range target.QueryAdd {
		postbackData[key] = value
	}

	return &OneSignalTarget{
		appID:         appID,
		apiKey:        apiKey,
		templateID:    templateID,
		includeImage:  includeImage,
		batchSize:     batchSize,
		useContents:   useContents,
		decodeTplArgs: decodeTplArgs,
		subtitle:      subtitle,
		language:      language,
		targets:       targets,
		customData:    customData,
		postbackData:  postbackData,
	}, nil
}

func (o *OneSignalTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	for _, category := range oneSignalCategoryOrder {
		targets := o.targets[category]
		if len(targets) == 0 {
			continue
		}
		payload := o.buildPayload(body, title, notifyType)
		payload[category] = targets[:minInt(len(targets), o.batchSize)]
		return o.buildRequest(payload)
	}

	return RequestSpec{}, fmt.Errorf("missing targets")
}

func (o *OneSignalTarget) Send(body, title string, notifyType NotifyType) error {
	payload := o.buildPayload(body, title, notifyType)
	sent := false

	for _, category := range oneSignalCategoryOrder {
		targets := o.targets[category]
		if len(targets) == 0 {
			continue
		}

		for idx := 0; idx < len(targets); idx += o.batchSize {
			end := idx + o.batchSize
			if end > len(targets) {
				end = len(targets)
			}

			payload[category] = targets[idx:end]
			spec, err := o.buildRequest(payload)
			if err != nil {
				return err
			}
			if err := SendRequest(spec); err != nil {
				return err
			}
			sent = true
		}
	}

	if !sent {
		return fmt.Errorf("missing targets")
	}

	return nil
}

func (o *OneSignalTarget) buildPayload(body, title string, notifyType NotifyType) map[string]any {
	payload := map[string]any{
		"app_id":            o.appID,
		"content_available": true,
	}

	if o.templateID != "" {
		payload["template_id"] = o.templateID
	}

	if o.templateID == "" || o.useContents {
		payload["contents"] = map[string]string{
			o.language: body,
		}
	}

	if len(o.customData) > 0 {
		payload["custom_data"] = o.customData
	}
	if len(o.postbackData) > 0 {
		payload["data"] = o.postbackData
	}

	if title != "" {
		payload["headings"] = map[string]string{
			o.language: title,
		}
	}

	if o.subtitle != "" {
		payload["subtitle"] = map[string]string{
			o.language: o.subtitle,
		}
	}

	if o.includeImage {
		payload["large_icon"] = appriseImageURL(notifyType, "72x72")
		payload["small_icon"] = appriseImageURL(notifyType, "32x32")
	}

	return payload
}

func (o *OneSignalTarget) buildRequest(payload map[string]any) (RequestSpec, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    onesignalURL,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/json; charset=utf-8",
			"Authorization": "Basic " + o.apiKey,
		},
		Body: string(data),
	}, nil
}

func normalizeOneSignalTargets(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	unique := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		unique[value] = struct{}{}
	}

	if len(unique) == 0 {
		return nil
	}

	sorted := make([]string, 0, len(unique))
	for value := range unique {
		sorted = append(sorted, value)
	}
	sort.Strings(sorted)
	return sorted
}

func decodeBase64Map(values map[string]string) map[string]string {
	decoded := map[string]string{}
	for key, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			decoded[key] = value
			continue
		}
		data, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			decoded[key] = value
			continue
		}
		decoded[key] = string(data)
	}
	return decoded
}
