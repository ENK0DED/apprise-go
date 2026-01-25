package notify

import (
	"encoding/json"
	"fmt"
	"strings"
)

const nextcloudTalkDefaultMessage = "Apprise"

type NextcloudTalkTarget struct {
	host      string
	port      int
	secure    bool
	user      string
	password  string
	targets   []string
	urlPrefix string
	headers   map[string]string
}

func NewNextcloudTalkTarget(target *ParsedURL) (*NextcloudTalkTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	user := strings.TrimSpace(target.User)
	if user == "" || strings.TrimSpace(target.Password) == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	targets := splitPath(target.Path)

	headers := cloneMap(target.QueryAdd)

	urlPrefix := strings.Trim(target.Query["url_prefix"], "/")

	return &NextcloudTalkTarget{
		host:      host,
		port:      target.Port,
		secure:    strings.EqualFold(target.Scheme, "nctalks"),
		user:      user,
		password:  target.Password,
		targets:   targets,
		urlPrefix: urlPrefix,
		headers:   headers,
	}, nil
}

func (n *NextcloudTalkTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(n.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	payload := map[string]string{
		"message": n.message(body, title),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	headers := n.buildHeaders()

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     n.buildURL(n.targets[0]),
		Headers: headers,
		Body:    string(data),
	}, nil
}

func (n *NextcloudTalkTarget) Send(body, title string, notifyType NotifyType) error {
	if len(n.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	payload := map[string]string{
		"message": n.message(body, title),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	headers := n.buildHeaders()

	for _, target := range n.targets {
		spec := RequestSpec{
			Method:  "POST",
			URL:     n.buildURL(target),
			Headers: headers,
			Body:    string(data),
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (n *NextcloudTalkTarget) buildHeaders() map[string]string {
	headers := map[string]string{
		"User-Agent":     "Apprise",
		"OCS-APIRequest": "true",
		"Accept":         "application/json",
		"Content-Type":   "application/json",
	}
	for key, value := range n.headers {
		headers[key] = value
	}
	headers["Authorization"] = basicAuthHeader(n.user, n.password)
	return headers
}

func (n *NextcloudTalkTarget) message(body, title string) string {
	if strings.TrimSpace(body) == "" {
		if strings.TrimSpace(title) == "" {
			return nextcloudTalkDefaultMessage
		}
		return title
	}
	if strings.TrimSpace(title) == "" {
		return nextcloudTalkDefaultMessage + "\r\n" + body
	}
	return title + "\r\n" + body
}

func (n *NextcloudTalkTarget) buildURL(target string) string {
	scheme := "http"
	if n.secure {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s", scheme, n.host)
	if n.port > 0 {
		base += fmt.Sprintf(":%d", n.port)
	}

	path := "/" + n.urlPrefix + "/ocs/v2.php/apps/spreed/api/v1/chat/" + target
	return base + path
}
