package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const enigma2DefaultTimeout = 13
const enigma2MinTimeout = -1

type Enigma2Target struct {
	host     string
	port     int
	secure   bool
	user     string
	password string
	fullpath string
	timeout  int
	headers  map[string]string
}

func NewEnigma2Target(target *ParsedURL) (*Enigma2Target, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	secure := strings.EqualFold(target.Scheme, "enigma2s")

	fullpath := target.Path
	if strings.TrimSpace(fullpath) == "" {
		fullpath = "/"
	}

	timeout := enigma2DefaultTimeout
	if rawTimeout := strings.TrimSpace(target.Query["timeout"]); rawTimeout != "" {
		if value, err := strconv.Atoi(rawTimeout); err == nil {
			if value < enigma2MinTimeout {
				value = enigma2MinTimeout
			}
			timeout = value
		}
	}

	headers := map[string]string{}
	for key, value := range target.QueryAdd {
		headers[key] = value
	}

	return &Enigma2Target{
		host:     host,
		port:     target.Port,
		secure:   secure,
		user:     target.User,
		password: target.Password,
		fullpath: fullpath,
		timeout:  timeout,
		headers:  headers,
	}, nil
}

func (e *Enigma2Target) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)

	params := url.Values{}
	params.Set("text", message)
	params.Set("type", strconv.Itoa(enigma2MessageType(notifyType)))
	params.Set("timeout", strconv.Itoa(e.timeout))

	requestURL := e.buildURL() + "?" + params.Encode()
	headers := map[string]string{
		"User-Agent": "Apprise",
	}
	for key, value := range e.headers {
		headers[key] = value
	}
	if e.user != "" {
		headers["Authorization"] = basicAuthHeader(e.user, e.password)
	}

	return RequestSpec{
		Method:  "GET",
		URL:     requestURL,
		Headers: headers,
	}, nil
}

func (e *Enigma2Target) Send(body, title string, notifyType NotifyType) error {
	spec, err := e.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (e *Enigma2Target) buildURL() string {
	scheme := "http"
	if e.secure {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s", scheme, e.host)
	if e.port > 0 {
		base += fmt.Sprintf(":%d", e.port)
	}

	trimmed := strings.TrimRight(e.fullpath, "/")
	if trimmed == "" {
		trimmed = ""
	}

	return base + trimmed + "/api/message"
}

func enigma2MessageType(notifyType NotifyType) int {
	switch notifyType {
	case NotifyWarning:
		return 2
	case NotifyFailure:
		return 3
	default:
		return 1
	}
}
