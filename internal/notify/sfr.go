package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const sfrURL = "https://www.dmc.sfr-sh.fr/DmcWS/1.5.8/JsonService/MessagesUnitairesWS/addSingleCall"

type SFRTarget struct {
	user     string
	password string
	spaceID  string
	lang     string
	sender   string
	media    string
	timeout  int
	voice    string
	targets  []string
}

type sfrAuthPayload struct {
	ServiceID       string `json:"serviceId"`
	ServicePassword string `json:"servicePassword"`
	SpaceID         string `json:"spaceId"`
	Lang            string `json:"lang"`
}

type sfrMessagePayload struct {
	Media    string `json:"media"`
	TextMsg  string `json:"textMsg"`
	To       string `json:"to"`
	From     string `json:"from"`
	Timeout  int    `json:"timeout"`
	TTSVoice string `json:"ttsVoice"`
}

func NewSFRTarget(target *ParsedURL) (*SFRTarget, error) {
	user := strings.TrimSpace(target.User)
	password := target.Password
	if user == "" || password == "" {
		return nil, fmt.Errorf("missing credentials")
	}

	spaceID := strings.TrimSpace(target.Host)
	if spaceID == "" {
		return nil, fmt.Errorf("missing space id")
	}

	lang := strings.TrimSpace(target.Query["lang"])
	if lang == "" {
		lang = "fr_FR"
	}

	sender := strings.TrimSpace(target.Query["sender"])
	if sender == "" {
		sender = strings.TrimSpace(target.Query["from"])
	}

	media := strings.TrimSpace(target.Query["media"])
	if media == "" {
		media = "SMSUnicode"
	}

	timeout := 2880
	if raw := strings.TrimSpace(target.Query["timeout"]); raw != "" {
		value, err := strconv.Atoi(raw)
		if err == nil {
			timeout = value
		}
	}

	voice := strings.TrimSpace(target.Query["voice"])
	if voice == "" {
		voice = "claire08s"
	}

	targets := []string{}
	for _, entry := range splitPath(target.Path) {
		if normalized, ok := normalizePhone(entry); ok {
			targets = append(targets, normalized)
		}
	}
	if toValue, ok := target.Query["to"]; ok && toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			if normalized, ok := normalizePhone(entry); ok {
				targets = append(targets, normalized)
			}
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	return &SFRTarget{
		user:     user,
		password: password,
		spaceID:  spaceID,
		lang:     lang,
		sender:   sender,
		media:    media,
		timeout:  timeout,
		voice:    voice,
		targets:  targets,
	}, nil
}

func (s *SFRTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	payload, err := s.buildPayload(s.targets[0], message)
	if err != nil {
		return RequestSpec{}, err
	}

	requestURL := sfrURL
	if encoded := payload.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	_ = notifyType

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"Accept": "*/*",
		},
	}, nil
}

func (s *SFRTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, target := range s.targets {
		payload, err := s.buildPayload(target, message)
		if err != nil {
			return err
		}

		requestURL := sfrURL
		if encoded := payload.Encode(); encoded != "" {
			requestURL += "?" + encoded
		}

		spec := RequestSpec{
			Method: "POST",
			URL:    requestURL,
			Headers: map[string]string{
				"Accept": "*/*",
			},
		}

		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (s *SFRTarget) buildPayload(target, message string) (url.Values, error) {
	authPayload := sfrAuthPayload{
		ServiceID:       s.user,
		ServicePassword: s.password,
		SpaceID:         s.spaceID,
		Lang:            s.lang,
	}
	authData, err := json.Marshal(authPayload)
	if err != nil {
		return nil, err
	}
	authText := addJSONSpaces(authData)

	messagePayload := sfrMessagePayload{
		Media:    s.media,
		TextMsg:  message,
		To:       target,
		From:     s.sender,
		Timeout:  s.timeout,
		TTSVoice: s.voice,
	}
	messageData, err := json.Marshal(messagePayload)
	if err != nil {
		return nil, err
	}
	messageText := addJSONSpaces(messageData)

	payload := url.Values{}
	payload.Set("authenticate", authText)
	payload.Set("messageUnitaire", messageText)
	return payload, nil
}

func addJSONSpaces(input []byte) string {
	var b strings.Builder
	b.Grow(len(input))

	inString := false
	escape := false
	for _, c := range input {
		if inString {
			b.WriteByte(c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
			b.WriteByte(c)
		case ':':
			b.WriteByte(':')
			b.WriteByte(' ')
		case ',':
			b.WriteByte(',')
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}
