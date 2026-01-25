package notify

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const homeAssistantDefaultPort = 8123

type HomeAssistantTarget struct {
	host        string
	port        int
	secure      bool
	user        string
	password    string
	accessToken string
	nid         string
	fullpath    string
}

func NewHomeAssistantTarget(target *ParsedURL) (*HomeAssistantTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	secure := strings.EqualFold(target.Scheme, "hassios")
	port := target.Port
	if port == 0 && !secure {
		port = homeAssistantDefaultPort
	}

	accessToken := strings.TrimSpace(target.Query["accesstoken"])
	fullpath := ""
	if accessToken == "" {
		parts := splitPath(target.Path)
		if len(parts) == 0 {
			return nil, fmt.Errorf("missing access token")
		}
		accessToken = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
		if len(parts) > 0 {
			fullpath = "/" + strings.Join(parts, "/")
		}
	}
	if accessToken == "" {
		return nil, fmt.Errorf("missing access token")
	}

	nid := strings.TrimSpace(target.Query["nid"])

	return &HomeAssistantTarget{
		host:        host,
		port:        port,
		secure:      secure,
		user:        strings.TrimSpace(target.User),
		password:    target.Password,
		accessToken: accessToken,
		nid:         nid,
		fullpath:    fullpath,
	}, nil
}

func (h *HomeAssistantTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := map[string]any{
		"title":           title,
		"message":         body,
		"notification_id": h.notificationID(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := map[string]string{
		"User-Agent":    "Apprise",
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + h.accessToken,
	}
	if h.user != "" {
		headers["Authorization"] = basicAuthHeader(h.user, h.password)
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     h.buildURL(),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (h *HomeAssistantTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := h.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (h *HomeAssistantTarget) buildURL() string {
	scheme := "http"
	if h.secure {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s", scheme, h.host)
	if h.port > 0 {
		base += fmt.Sprintf(":%d", h.port)
	}

	path := strings.TrimRight(h.fullpath, "/")
	return base + path + "/api/services/persistent_notification/create"
}

func (h *HomeAssistantTarget) notificationID() string {
	if h.nid != "" {
		return h.nid
	}
	return newUUIDv4()
}

func newUUIDv4() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(buf[0:4]),
		hex.EncodeToString(buf[4:6]),
		hex.EncodeToString(buf[6:8]),
		hex.EncodeToString(buf[8:10]),
		hex.EncodeToString(buf[10:16]),
	)
}
