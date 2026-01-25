package notify

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var joinGroupRe = regexp.MustCompile(`^(group\.)?(all|android|chrome|windows10|phone|tablet|pc)$`)
var joinDeviceRe = regexp.MustCompile(`^[a-z0-9]{32}$`)

type JoinTarget struct {
	apiKey       string
	targets      []string
	includeImage bool
	priority     int
}

func NewJoinTarget(target *ParsedURL) (*JoinTarget, error) {
	apiKey := strings.TrimSpace(target.User)
	if apiKey == "" {
		apiKey = strings.TrimSpace(target.Host)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing apikey")
	}

	targets := []string{}
	if target.User != "" && target.Host != "" {
		targets = append(targets, target.Host)
	}
	targets = append(targets, splitPath(target.Path)...)
	if toValue, ok := target.Query["to"]; ok && strings.TrimSpace(toValue) != "" {
		targets = append(targets, parseDelimitedList(toValue)...)
	}
	if len(targets) == 0 {
		targets = append(targets, "group.all")
	}

	priority := 0
	if rawPriority := strings.TrimSpace(target.Query["priority"]); rawPriority != "" {
		switch strings.ToLower(rawPriority) {
		case "low", "l", "-2":
			priority = -2
		case "moderate", "m", "-1":
			priority = -1
		case "normal", "n", "0":
			priority = 0
		case "high", "h", "1":
			priority = 1
		case "emergency", "e", "2":
			priority = 2
		}
	}

	return &JoinTarget{
		apiKey:       apiKey,
		targets:      targets,
		includeImage: parseBool(target.Query["image"], true),
		priority:     priority,
	}, nil
}

func (j *JoinTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := j.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (j *JoinTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(j.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	target := j.targets[0]
	args := url.Values{}
	args.Set("apikey", j.apiKey)
	args.Set("priority", fmt.Sprintf("%d", j.priority))
	args.Set("title", title)
	args.Set("text", body)

	if joinDeviceRe.MatchString(target) || joinGroupRe.MatchString(target) {
		if joinGroupRe.MatchString(target) && !strings.HasPrefix(target, "group.") {
			target = "group." + target
		}
		args.Set("deviceId", target)
	} else {
		args.Set("deviceNames", target)
	}

	if j.includeImage {
		args.Set("icon", appriseImageURL(notifyType, "72x72"))
	}

	u := url.URL{
		Scheme:   "https",
		Host:     "joinjoaomgcd.appspot.com",
		Path:     "/_ah/api/messaging/v1/sendPush",
		RawQuery: args.Encode(),
	}

	return RequestSpec{
		Method: "POST",
		URL:    u.String(),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: "",
	}, nil
}
