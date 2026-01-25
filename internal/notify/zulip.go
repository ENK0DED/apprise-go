package notify

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const zulipDefaultHostname = "zulipchat.com"

var zulipTargetDelims = regexp.MustCompile(`[ \t\r\n,#\\/]+`)

type ZulipTarget struct {
	organization string
	hostname     string
	botname      string
	token        string
	targets      []string
}

func NewZulipTarget(target *ParsedURL) (*ZulipTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing organization")
	}

	organization, hostname := splitZulipHost(host)

	botname := strings.TrimSpace(target.User)
	if botname == "" {
		return nil, fmt.Errorf("missing botname")
	}
	if strings.HasSuffix(strings.ToLower(botname), "-bot") {
		botname = botname[:len(botname)-4]
	}

	segments := splitPath(target.Path)
	if len(segments) == 0 {
		return nil, fmt.Errorf("missing token")
	}
	token := segments[0]

	targets := splitZulipTargets(strings.Join(segments[1:], "/"))
	if targetValue, ok := target.Query["to"]; ok && strings.TrimSpace(targetValue) != "" {
		targets = append(targets, splitZulipTargets(targetValue)...)
	}
	if len(targets) == 0 {
		targets = []string{"general"}
	}

	return &ZulipTarget{
		organization: organization,
		hostname:     hostname,
		botname:      botname,
		token:        token,
		targets:      targets,
	}, nil
}

func (z *ZulipTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := z.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (z *ZulipTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(z.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	target := z.targets[0]

	payload := url.Values{}
	payload.Set("subject", title)
	payload.Set("content", body)
	if strings.Contains(target, "@") {
		payload.Set("type", "private")
		payload.Set("to", target)
	} else {
		payload.Set("type", "stream")
		payload.Set("to", target)
	}

	authUser := fmt.Sprintf("%s-bot@%s.%s", z.botname, z.organization, z.hostname)
	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
		"Authorization": basicAuthHeader(
			authUser,
			z.token,
		),
	}

	url := fmt.Sprintf("https://%s.%s/api/v1/messages", z.organization, z.hostname)

	return RequestSpec{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    payload.Encode(),
	}, nil
}

func splitZulipHost(host string) (string, string) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return "", zulipDefaultHostname
	}
	if idx := strings.Index(trimmed, "."); idx != -1 {
		return trimmed[:idx], trimmed[idx+1:]
	}
	return trimmed, zulipDefaultHostname
}

func splitZulipTargets(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	parts := zulipTargetDelims.Split(trimmed, -1)
	targets := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		decoded, err := url.PathUnescape(part)
		if err == nil {
			part = decoded
		}
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		targets = append(targets, part)
	}

	if len(targets) == 0 {
		return nil
	}
	return targets
}
