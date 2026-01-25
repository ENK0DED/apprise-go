package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const googleChatURL = "https://chat.googleapis.com/v1/spaces/%s/messages"

const googleChatReplyOption = "REPLY_MESSAGE_FALLBACK_TO_NEW_THREAD"

type GoogleChatTarget struct {
	workspace string
	key       string
	token     string
	thread    string
}

func NewGoogleChatTarget(target *ParsedURL) (*GoogleChatTarget, error) {
	segments := splitPathSegments(target.Path)

	workspace := strings.TrimSpace(target.Host)
	if raw := strings.TrimSpace(target.Query["workspace"]); raw != "" {
		workspace = raw
	}

	key := ""
	if len(segments) > 0 {
		key = segments[0]
	}
	if raw := strings.TrimSpace(target.Query["key"]); raw != "" {
		key = raw
	}

	token := ""
	if len(segments) > 1 {
		token = segments[1]
	}
	if raw := strings.TrimSpace(target.Query["token"]); raw != "" {
		token = raw
	}

	thread := ""
	if len(segments) > 2 {
		thread = segments[2]
	}
	if raw := strings.TrimSpace(target.Query["thread"]); raw != "" {
		thread = raw
	} else if raw := strings.TrimSpace(target.Query["threadkey"]); raw != "" {
		thread = raw
	}

	if workspace == "" || key == "" || token == "" {
		return nil, fmt.Errorf("missing workspace, key, or token")
	}

	return &GoogleChatTarget{
		workspace: workspace,
		key:       key,
		token:     token,
		thread:    thread,
	}, nil
}

func (g *GoogleChatTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\r\n" + body
	}

	payload := map[string]any{
		"text": message,
	}

	params := url.Values{}
	params.Set("token", g.token)
	params.Set("key", g.key)

	if g.thread != "" {
		params.Set("messageReplyOption", googleChatReplyOption)
		payload["thread"] = map[string]string{
			"thread_key": g.thread,
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	_ = notifyType

	requestURL := fmt.Sprintf(googleChatURL, g.workspace)
	if encoded := params.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Accept":       "*/*",
			"Content-Type": "application/json; charset=utf-8",
		},
		Body: string(data),
	}, nil
}

func (g *GoogleChatTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := g.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func splitPathSegments(rawPath string) []string {
	path := strings.Trim(rawPath, "/")
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		decoded, err := url.PathUnescape(part)
		if err != nil {
			decoded = part
		}
		decoded = strings.TrimSpace(decoded)
		if decoded != "" {
			segments = append(segments, decoded)
		}
	}
	return segments
}
