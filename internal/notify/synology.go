package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type SynologyTarget struct {
	host     string
	port     int
	secure   bool
	user     string
	password string
	token    string
	fullpath string
	fileURL  string
	headers  map[string]string
}

func NewSynologyTarget(target *ParsedURL) (*SynologyTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	token := strings.TrimSpace(target.Query["token"])
	fullpath := ""
	if token == "" {
		entries := splitPath(target.Path)
		if len(entries) == 0 {
			return nil, fmt.Errorf("missing token")
		}
		token = entries[0]
		entries = entries[1:]
		if len(entries) > 0 {
			fullpath = "/" + strings.Join(entries, "/")
		}
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	fileURL := strings.TrimSpace(target.Query["file_url"])

	headers := map[string]string{}
	for key, value := range target.QueryAdd {
		headers[key] = value
	}

	return &SynologyTarget{
		host:     host,
		port:     target.Port,
		secure:   strings.EqualFold(target.Scheme, "synologys"),
		user:     strings.TrimSpace(target.User),
		password: target.Password,
		token:    token,
		fullpath: fullpath,
		fileURL:  fileURL,
		headers:  headers,
	}, nil
}

func (s *SynologyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := mergeTitleBody(title, body)

	textValue, err := json.Marshal(message)
	if err != nil {
		return RequestSpec{}, err
	}

	payloadData := []byte{}
	if s.fileURL == "" {
		payloadData = []byte(fmt.Sprintf("{\"text\": %s}", textValue))
	} else {
		fileValue, err := json.Marshal(s.fileURL)
		if err != nil {
			return RequestSpec{}, err
		}
		payloadData = []byte(fmt.Sprintf("{\"text\": %s, \"file_url\": %s}", textValue, fileValue))
	}

	params := url.Values{}
	params.Set("api", "SYNO.Chat.External")
	params.Set("method", "incoming")
	params.Set("version", "2")
	params.Set("token", s.token)

	requestURL := s.buildURL() + "?" + params.Encode()

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Content-Type": "application/x-www-form-urlencoded",
		"Accept":       "*/*",
	}
	for key, value := range s.headers {
		headers[key] = value
	}
	if s.user != "" {
		headers["Authorization"] = basicAuthHeader(s.user, s.password)
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     requestURL,
		Headers: headers,
		Body:    "payload=" + string(payloadData),
	}, nil
}

func (s *SynologyTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := s.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (s *SynologyTarget) buildURL() string {
	scheme := "http"
	if s.secure {
		scheme = "https"
	}

	base := fmt.Sprintf("%s://%s", scheme, s.host)
	if s.port > 0 {
		base += fmt.Sprintf(":%d", s.port)
	}

	path := strings.TrimRight(s.fullpath, "/")
	return base + path + "/webapi/entry.cgi"
}
