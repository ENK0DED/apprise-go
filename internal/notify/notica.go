package notify

import (
	"fmt"
	"strings"
)

const noticaOfficialURL = "https://notica.us/?%s"

type NoticaTarget struct {
	mode    string
	scheme  string
	host    string
	port    int
	path    string
	token   string
	headers map[string]string
	user    string
	pass    string
}

func NewNoticaTarget(target *ParsedURL) (*NoticaTarget, error) {
	segments := splitPath(target.Path)
	mode := "official"
	token := ""
	host := ""
	path := ""

	if len(segments) == 0 {
		mode = "official"
		token = target.Host
	} else {
		mode = "selfhosted"
		token = segments[len(segments)-1]
		host = target.Host
		if len(segments) > 1 {
			path = "/" + strings.Join(segments[:len(segments)-1], "/") + "/"
		} else {
			path = "/"
		}
	}

	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	scheme := "http"
	if strings.ToLower(target.Scheme) == "noticas" {
		scheme = "https"
	}

	return &NoticaTarget{
		mode:    mode,
		scheme:  scheme,
		host:    host,
		port:    target.Port,
		path:    path,
		token:   token,
		headers: cloneMap(target.QueryAdd),
		user:    target.User,
		pass:    target.Password,
	}, nil
}

func (n *NoticaTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded",
	}

	if n.mode == "selfhosted" {
		for key, value := range n.headers {
			headers[key] = value
		}
		if n.user != "" {
			headers["Authorization"] = basicAuthHeader(n.user, n.pass)
		}
	}

	url := ""
	if n.mode == "official" {
		url = fmt.Sprintf(noticaOfficialURL, n.token)
	} else {
		host := n.host
		if n.port != 0 {
			host = fmt.Sprintf("%s:%d", host, n.port)
		}
		url = fmt.Sprintf("%s://%s%s?token=%s", n.scheme, host, n.path, n.token)
	}

	_ = title
	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     url,
		Headers: headers,
		Body:    "d:" + body,
	}, nil
}

func (n *NoticaTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := n.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}
