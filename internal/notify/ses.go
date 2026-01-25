package notify

import (
	"encoding/base64"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"time"
)

const sesServiceName = "ses"
const sesDefaultFromName = "Apprise Notifications"

var awsRegionPattern = regexp.MustCompile(`^[A-Za-z]{2}-[A-Za-z-]+-[0-9]+$`)

type SESTarget struct {
	accessKey string
	secretKey string
	region    string
	fromEmail string
	fromName  string
	targets   []string
}

func NewSESTarget(target *ParsedURL) (*SESTarget, error) {
	fromEmail := ""
	if target.User != "" && target.Host != "" {
		fromEmail = strings.TrimSpace(target.User) + "@" + strings.TrimSpace(target.Host)
	} else if strings.Contains(target.Host, "@") {
		fromEmail = strings.TrimSpace(target.Host)
	}
	if !isSimpleEmail(fromEmail) {
		return nil, fmt.Errorf("invalid from email")
	}

	entries := splitPath(target.Path)
	if len(entries) < 2 {
		return nil, fmt.Errorf("missing credentials")
	}

	accessKey := strings.TrimSpace(entries[0])
	rest := entries[1:]

	secretParts := []string{}
	region := ""
	index := 0
	for i, entry := range rest {
		if awsRegionPattern.MatchString(entry) {
			region = normalizeAWSRegion(entry)
			index = i + 1
			break
		}
		secretParts = append(secretParts, entry)
	}
	if accessKey == "" {
		return nil, fmt.Errorf("missing access key")
	}
	if region == "" {
		return nil, fmt.Errorf("missing region")
	}

	secretKey := strings.TrimSpace(strings.Join(secretParts, "/"))
	if rawSecret := strings.TrimSpace(target.Query["secret"]); rawSecret != "" {
		secretKey = rawSecret
	}
	if secretKey == "" {
		return nil, fmt.Errorf("missing secret key")
	}

	if rawAccess := strings.TrimSpace(target.Query["access"]); rawAccess != "" {
		accessKey = rawAccess
	}
	if rawRegion := strings.TrimSpace(target.Query["region"]); rawRegion != "" {
		region = normalizeAWSRegion(rawRegion)
	}

	targets := []string{}
	for _, entry := range rest[index:] {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if isSimpleEmail(entry) {
			targets = append(targets, entry)
		}
	}
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			if isSimpleEmail(entry) {
				targets = append(targets, entry)
			}
		}
	}
	if len(targets) == 0 {
		targets = append(targets, fromEmail)
	}

	fromName := strings.TrimSpace(target.Query["name"])
	if fromName == "" {
		fromName = sesDefaultFromName
	}

	return &SESTarget{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		fromEmail: fromEmail,
		fromName:  fromName,
		targets:   targets,
	}, nil
}

func (s *SESTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if len(s.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}
	payload := s.buildPayload(body, title, s.targets[0])
	return RequestSpec{
		Method:  "POST",
		URL:     s.notifyURL(),
		Headers: s.signer().headers(payload, fixedTime()),
		Body:    payload,
	}, nil
}

func (s *SESTarget) Send(body, title string, notifyType NotifyType) error {
	if len(s.targets) == 0 {
		return fmt.Errorf("missing targets")
	}

	for _, target := range s.targets {
		payload := s.buildPayload(body, title, target)
		spec := RequestSpec{
			Method:  "POST",
			URL:     s.notifyURL(),
			Headers: s.signer().headers(payload, fixedTime()),
			Body:    payload,
		}
		if err := SendRequest(spec); err != nil {
			return err
		}
	}

	_ = notifyType
	return nil
}

func (s *SESTarget) notifyURL() string {
	return fmt.Sprintf("https://email.%s.amazonaws.com/", s.region)
}

func (s *SESTarget) signer() awsSigV4 {
	return awsSigV4{
		accessKey: s.accessKey,
		secretKey: s.secretKey,
		region:    s.region,
		service:   sesServiceName,
		host:      fmt.Sprintf("email.%s.amazonaws.com", s.region),
	}
}

func (s *SESTarget) buildPayload(body, title, target string) string {
	raw := buildSESMIME(s.fromName, s.fromEmail, target, body, title, fixedTime())
	message := base64.StdEncoding.EncodeToString([]byte(raw))

	pairs := []formPair{
		{key: "Action", value: "SendRawEmail"},
		{key: "Version", value: "2010-12-01"},
		{key: "RawMessage.Data", value: message},
		{key: "Destinations.member.1", value: target},
		{key: "Source", value: sesSourceValue(s.fromName, s.fromEmail)},
	}
	return encodeFormPairs(pairs)
}

func buildSESMIME(fromName, fromEmail, toEmail, body, title string, now time.Time) string {
	subject := ""
	if strings.TrimSpace(title) != "" {
		subject = encodeRFC2047(title)
	}
	from := formatMIMEAddress(fromName, fromEmail)
	to := formatMIMEAddress("", toEmail)
	date := now.UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")

	headers := []string{
		`Content-Type: text/html; charset="utf-8"`,
		"MIME-Version: 1.0",
		"Content-Transfer-Encoding: quoted-printable",
		fmt.Sprintf("Subject: %s", subject),
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		"Cc: ",
		fmt.Sprintf("Date: %s", date),
		"X-Application: Apprise",
	}

	encodedBody, err := encodeQuotedPrintable(body)
	if err != nil {
		encodedBody = body
	}

	return strings.Join(headers, "\n") + "\n\n" + encodedBody
}

func formatMIMEAddress(name, email string) string {
	if strings.TrimSpace(name) == "" {
		return email
	}
	if isASCII(name) {
		return fmt.Sprintf("%s <%s>", name, email)
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", name), email)
}

func sesSourceValue(name, email string) string {
	return fmt.Sprintf(
		"%s <%s>",
		awsQuote(name, " "),
		awsQuote(email, "@ "),
	)
}

func awsQuote(value, safe string) string {
	if value == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range value {
		if isSafeAWSChar(r, safe) {
			b.WriteRune(r)
			continue
		}
		b.WriteString(fmt.Sprintf("%%%02X", r))
	}
	return b.String()
}

func isSafeAWSChar(r rune, safe string) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '-', '_', '.', '~':
		return true
	}
	return strings.ContainsRune(safe, r)
}

func normalizeAWSRegion(region string) string {
	parts := strings.Split(region, "-")
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	return strings.Join(parts, "-")
}

func encodeRFC2047(value string) string {
	encoded := mime.QEncoding.Encode("utf-8", value)
	if strings.HasPrefix(encoded, "=?") {
		return encoded
	}

	var b strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		switch {
		case ch == ' ':
			b.WriteByte('_')
		case (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9'):
			b.WriteByte(ch)
		default:
			b.WriteString(fmt.Sprintf("=%02X", ch))
		}
	}
	return "=?utf-8?q?" + b.String() + "?="
}

func encodeQuotedPrintable(value string) (string, error) {
	var b strings.Builder
	writer := quotedprintable.NewWriter(&b)
	if _, err := writer.Write([]byte(value)); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func isASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}
