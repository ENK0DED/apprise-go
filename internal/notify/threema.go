package notify

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const threemaNotifyURL = "https://msgapi.threema.ch/send_simple"

type threemaRecipient struct {
	key   string
	value string
}

type ThreemaTarget struct {
	user       string
	secret     string
	recipients []threemaRecipient
}

func NewThreemaTarget(target *ParsedURL) (*ThreemaTarget, error) {
	user := strings.TrimSpace(target.User)
	if from := strings.TrimSpace(target.Query["from"]); from != "" {
		user = from
	} else if gwid := strings.TrimSpace(target.Query["gwid"]); gwid != "" {
		user = gwid
	}

	secret := strings.TrimSpace(target.Query["secret"])
	if secret == "" {
		secret = strings.TrimSpace(target.Host)
	}

	if user == "" {
		return nil, fmt.Errorf("missing gateway id")
	}
	if secret == "" {
		return nil, fmt.Errorf("missing secret")
	}

	targets := splitPath(target.Path)
	if toRaw := strings.TrimSpace(target.Query["to"]); toRaw != "" {
		targets = append(targets, parseDelimitedList(toRaw)...)
	}

	recipients := make([]threemaRecipient, 0, len(targets))
	for _, entry := range targets {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if len(entry) == 8 {
			recipients = append(recipients, threemaRecipient{key: "to", value: entry})
			continue
		}
		if isSimpleEmail(entry) {
			recipients = append(recipients, threemaRecipient{key: "email", value: entry})
			continue
		}
		if normalized, ok := normalizePhone(entry); ok {
			recipients = append(recipients, threemaRecipient{key: "phone", value: normalized})
			continue
		}
	}

	if len(recipients) == 0 {
		return nil, fmt.Errorf("missing targets")
	}

	sort.Slice(recipients, func(i, j int) bool {
		if recipients[i].key != recipients[j].key {
			return recipients[i].key < recipients[j].key
		}
		return recipients[i].value < recipients[j].value
	})

	return &ThreemaTarget{
		user:       user,
		secret:     secret,
		recipients: recipients,
	}, nil
}

func (t *ThreemaTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(t.recipients) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	spec := t.buildSpec(message, t.recipients[0])
	_ = notifyType
	return spec, nil
}

func (t *ThreemaTarget) Send(body, title string, notifyType NotifyType) error {
	if len(t.recipients) == 0 {
		return fmt.Errorf("missing targets")
	}

	message := mergeTitleBody(title, body)
	for _, recipient := range t.recipients {
		spec := t.buildSpec(message, recipient)
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType

	return nil
}

func (t *ThreemaTarget) buildSpec(message string, recipient threemaRecipient) RequestSpec {
	values := url.Values{}
	values.Set("secret", t.secret)
	values.Set("from", t.user)
	values.Set("text", message)
	values.Set(recipient.key, recipient.value)

	return RequestSpec{
		Method: "POST",
		URL:    threemaNotifyURL + "?" + values.Encode(),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
			"Accept":       "*/*",
		},
	}
}
