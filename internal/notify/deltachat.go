package notify

import "strings"

// Delta Chat uses Email's SMTP transport and adds its chat protocol headers.
type DeltaChatTarget struct{ mail *MailtoTarget }

func NewDeltaChatTarget(p *ParsedURL) (*DeltaChatTarget, error) {
	copyURL := *p
	copyURL.Scheme = "mailto"
	if p.Scheme == "deltachats" {
		copyURL.Scheme = "mailtos"
	}
	copyURL.Query = cloneMap(p.Query)
	copyURL.Query["format"] = "text"
	copyURL.resolvedNotifyFormat = "text"
	target, err := NewMailtoTarget(&copyURL)
	if err != nil {
		return nil, err
	}
	target.headers["Chat-Version"] = "1.0"
	return &DeltaChatTarget{mail: target}, nil
}

func (t *DeltaChatTarget) Send(body, title string, kind NotifyType) error {
	return t.SendWithAttachments(body, title, kind, nil)
}

func (t *DeltaChatTarget) SendWithAttachments(body, title string, kind NotifyType, attachments []Attachment) error {
	body = mergeTitleBody(title, body)
	excerpt := []rune(strings.Join(strings.Fields(body), " "))
	if len(excerpt) > 50 {
		excerpt = append(excerpt[:50], '.', '.', '.')
	}
	subject := "Chat:"
	if len(excerpt) > 0 {
		subject += " " + string(excerpt)
	}
	return t.mail.SendWithAttachments(body, subject, kind, attachments)
}
