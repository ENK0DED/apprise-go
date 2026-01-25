package notify

import (
	"encoding/json"
	"fmt"
)

const guildedWebhookBase = "https://media.guilded.gg/webhooks"

type GuildedTarget struct {
	webhookID    string
	webhookToken string
	username     string
	tts          bool
	avatar       bool
	avatarURL    string
}

func NewGuildedTarget(target *ParsedURL) (*GuildedTarget, error) {
	webhookID := target.Host
	segments := splitPath(target.Path)
	if webhookID == "" || len(segments) == 0 {
		return nil, fmt.Errorf("missing webhook credentials")
	}
	webhookToken := segments[0]

	tts := parseBoolWithDefault(target.Query["tts"], false)
	avatar := true
	if rawAvatar, ok := target.Query["avatar"]; ok {
		avatar = parseBoolWithDefault(rawAvatar, true)
	}
	avatarURL := ""
	if rawAvatarURL, ok := target.Query["avatar_url"]; ok && rawAvatarURL != "" {
		avatarURL = rawAvatarURL
	}

	return &GuildedTarget{
		webhookID:    webhookID,
		webhookToken: webhookToken,
		username:     target.User,
		tts:          tts,
		avatar:       avatar,
		avatarURL:    avatarURL,
	}, nil
}

func (g *GuildedTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"tts":  g.tts,
		"wait": !g.tts,
	}

	if g.avatar {
		if g.avatarURL != "" {
			payload["avatar_url"] = g.avatarURL
		} else {
			payload["avatar_url"] = defaultImageURL(notifyType)
		}
	}

	if g.username != "" {
		payload["username"] = g.username
	}

	if body != "" {
		if title == "" {
			payload["content"] = body
		} else {
			payload["content"] = title + "\r\n" + body
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/json; charset=utf-8",
	}

	url := fmt.Sprintf("%s/%s/%s", guildedWebhookBase, g.webhookID, g.webhookToken)

	return RequestSpec{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (g *GuildedTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := g.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
