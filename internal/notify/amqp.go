package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	amqp10 "github.com/Azure/go-amqp"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

type AMQPTarget struct {
	deliveryTracker
	parsed                      *ParsedURL
	version, exchange, vhost    string
	persistent, mandatory       bool
	priority                    *uint8
	ttl                         *uint32
	targets                     []string
	connectTimeout, readTimeout time.Duration
}

var amqpVersionPattern = regexp.MustCompile(`(?i)^v?[0-9]+([.-][0-9]+)*$`)
var amqpVersionDigits = regexp.MustCompile(`[^0-9]`)

func NewAMQPTarget(p *ParsedURL) (*AMQPTarget, error) {
	if !isValidHostname(p.Host) {
		return nil, fmt.Errorf("invalid AMQP hostname")
	}
	if p.HasPort && (p.Port < 1 || p.Port > 65535) {
		return nil, fmt.Errorf("invalid AMQP port")
	}
	t := &AMQPTarget{parsed: p, version: "0.9.1", exchange: p.Query["exchange"], vhost: p.Query["vhost"], persistent: parseBoolWithDefault(p.Query["persistent"], true), mandatory: parseBoolWithDefault(p.Query["mandatory"], true), connectTimeout: 4 * time.Second, readTimeout: 4 * time.Second}
	version := p.Query["version"]
	if version == "" {
		version = p.Query["ver"]
	}
	if version != "" {
		version = strings.TrimSpace(version)
		if !amqpVersionPattern.MatchString(version) {
			return nil, fmt.Errorf("invalid AMQP version")
		}
		t.version = map[string]string{"0": "0.9.1", "09": "0.9.1", "091": "0.9.1", "1": "1.0", "10": "1.0"}[amqpVersionDigits.ReplaceAllString(version, "")]
		if t.version == "" {
			return nil, fmt.Errorf("invalid AMQP version")
		}
	}
	if t.version == "1.0" {
		t.exchange = ""
	} else if len(t.exchange) > 255 || len(t.vhost) > 255 {
		return nil, fmt.Errorf("AMQP exchange or vhost exceeds 255 bytes")
	}
	if raw := p.Query["priority"]; raw != "" {
		value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid AMQP priority")
		}
		priority := uint8(value)
		t.priority = &priority
	}
	if raw := p.Query["ttl"]; raw != "" {
		value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid AMQP TTL")
		}
		ttl := uint32(value)
		t.ttl = &ttl
	}
	for _, entry := range sortedUniqueTargets(append(splitPath(p.Path), parseDelimitedList(p.Query["to"])...)) {
		if t.version != "0.9.1" || len(entry) <= 255 {
			t.targets = append(t.targets, entry)
		}
	}
	for name, destination := range map[string]*time.Duration{"cto": &t.connectTimeout, "rto": &t.readTimeout} {
		if value, err := strconv.ParseFloat(p.Query[name], 64); err == nil && value > 0 && !math.IsInf(value, 0) {
			*destination = time.Duration(value * float64(time.Second))
		}
	}
	return t, nil
}

func (t *AMQPTarget) address() string {
	port := 5672
	if t.parsed.Scheme == "amqps" {
		port = 5671
	}
	if t.parsed.HasPort {
		port = t.parsed.Port
	}
	return net.JoinHostPort(t.parsed.Host, strconv.Itoa(port))
}
func (t *AMQPTarget) contentType() string {
	format := targetNotifyFormat(t.parsed)
	if format != "html" && format != "markdown" {
		format = "plain"
	}
	return "text/" + format + "; charset=utf-8"
}
func (t *AMQPTarget) tlsConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: t.parsed.Host, InsecureSkipVerify: !parseBoolWithDefault(t.parsed.Query["verify"], true)} // #nosec G402 -- verify=no explicitly selects upstream's unverified TLS mode.
}

func (t *AMQPTarget) Send(body, title string, _ NotifyType) error {
	targets := t.targets
	if len(targets) == 0 && t.exchange != "" {
		targets = []string{""}
	}
	if len(targets) == 0 {
		return fmt.Errorf("no AMQP targets")
	}
	pending := make([]string, 0, len(targets))
	for _, target := range targets {
		if !t.isDelivered(target) {
			pending = append(pending, target)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	body = mergeTitleBody(title, body)
	if t.version == "1.0" {
		return t.send10(body, pending)
	}
	return t.send091(body, pending)
}

// Every read and write is bounded, including the protocol handshake and
// publisher confirmations. A connected broker cannot stall the caller forever.
type amqpDeadlineConn struct {
	net.Conn
	timeout  time.Duration
	deadline time.Time
}

func (c *amqpDeadlineConn) nextDeadline() time.Time {
	deadline := time.Now().Add(c.timeout)
	if !c.deadline.IsZero() && c.deadline.Before(deadline) {
		return c.deadline
	}
	return deadline
}
func amqpContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(timeout)
	if notification := currentNotificationDeadline(); !notification.IsZero() && notification.Before(deadline) {
		deadline = notification
	}
	return context.WithDeadline(context.Background(), deadline)
}
func (c *amqpDeadlineConn) Read(data []byte) (int, error) {
	if err := c.SetReadDeadline(c.nextDeadline()); err != nil {
		return 0, err
	}
	return c.Conn.Read(data)
}
func (c *amqpDeadlineConn) Write(data []byte) (int, error) {
	if err := c.SetWriteDeadline(c.nextDeadline()); err != nil {
		return 0, err
	}
	return c.Conn.Write(data)
}

func (t *AMQPTarget) send091(body string, targets []string) error {
	user, password := t.parsed.User, t.parsed.Password
	if user == "" {
		user, password = "guest", "guest"
	}
	scheme := "amqp"
	if t.parsed.Scheme == "amqps" {
		scheme = "amqps"
	}
	address := (&url.URL{Scheme: scheme, Host: t.address(), User: url.UserPassword(user, password)}).String()
	vhost := t.vhost
	if vhost == "" {
		vhost = "/"
	}
	config := amqp091.Config{Vhost: vhost, Heartbeat: time.Duration(math.Ceil(t.readTimeout.Seconds())) * time.Second, TLSClientConfig: t.tlsConfig(), Dial: func(network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: t.connectTimeout, Deadline: currentNotificationDeadline()}).Dial(network, address)
		if err != nil {
			return nil, err
		}
		return &amqpDeadlineConn{Conn: conn, timeout: t.readTimeout, deadline: currentNotificationDeadline()}, nil
	}}
	connection, err := amqp091.DialConfig(address, config)
	if err != nil {
		return err
	}
	defer connection.Close()
	var channel *amqp091.Channel
	var returned <-chan amqp091.Return
	var outcome sendOutcome
	message := amqp091.Publishing{ContentType: t.contentType(), DeliveryMode: 1, Timestamp: time.Unix(fixedTime().Unix(), 0), AppId: "Apprise", Body: []byte(body)}
	if t.persistent {
		message.DeliveryMode = 2
	}
	if t.priority != nil {
		message.Priority = *t.priority
	}
	if t.ttl != nil {
		message.Expiration = strconv.FormatUint(uint64(*t.ttl), 10)
	}
	if len(t.parsed.QueryAdd) > 0 {
		message.Headers = amqp091.Table{}
		for name, value := range t.parsed.QueryAdd {
			message.Headers[name] = value
		}
	}
	for _, target := range targets {
		send := func() error {
			if channel == nil || channel.IsClosed() {
				var err error
				channel, err = connection.Channel()
				if err != nil {
					return err
				}
				if err := channel.Confirm(false); err != nil {
					return err
				}
				returned = channel.NotifyReturn(make(chan amqp091.Return, 1))
			}
			ctx, cancel := amqpContext(t.readTimeout)
			defer cancel()
			confirmation, err := channel.PublishWithDeferredConfirmWithContext(ctx, t.exchange, target, t.mandatory, false, message)
			if err != nil {
				return err
			}
			if confirmation == nil {
				return fmt.Errorf("AMQP broker did not enable publisher confirms")
			}
			accepted, err := confirmation.WaitContext(ctx)
			if err != nil {
				return err
			}
			if !accepted {
				return fmt.Errorf("AMQP broker rejected message")
			}
			select {
			case result, open := <-returned:
				if open {
					return fmt.Errorf("AMQP unroutable message: %d %s", result.ReplyCode, result.ReplyText)
				}
			default:
			}
			return nil
		}
		err := send()
		outcome.record(err)
		if err == nil {
			t.markDelivered(target)
		}
	}
	return outcome.err()
}

func (t *AMQPTarget) message10(body string) *amqp10.Message {
	message := amqp10.NewMessage([]byte(body))
	priority := uint8(4)
	if t.priority != nil {
		priority = *t.priority
	}
	message.Header = &amqp10.MessageHeader{Durable: t.persistent, Priority: priority}
	if t.ttl != nil {
		message.Header.TTL = time.Duration(*t.ttl) * time.Millisecond
		if *t.ttl == 0 {
			message.Header.TTL = time.Nanosecond
		}
	} // A nonzero duration encodes an explicit zero-millisecond TTL rather than an absent field.
	contentType := t.contentType()
	created := time.Unix(fixedTime().Unix(), 0)
	message.Properties = &amqp10.MessageProperties{ContentType: &contentType, CreationTime: &created}
	if len(t.parsed.QueryAdd) > 0 {
		message.ApplicationProperties = map[string]any{}
		for name, value := range t.parsed.QueryAdd {
			message.ApplicationProperties[name] = value
		}
	}
	return message
}
func (t *AMQPTarget) send10(body string, targets []string) error {
	options := &amqp10.ConnOptions{IdleTimeout: t.readTimeout, WriteTimeout: t.readTimeout, TLSConfig: t.tlsConfig(), SASLType: amqp10.SASLTypeAnonymous()}
	if t.parsed.User != "" {
		options.SASLType = amqp10.SASLTypePlain(t.parsed.User, t.parsed.Password)
	}
	if t.vhost != "" {
		options.HostName = t.vhost
	}
	scheme := "amqp"
	if t.parsed.Scheme == "amqps" {
		scheme = "amqps"
	}
	ctx, cancel := amqpContext(t.connectTimeout)
	connection, err := amqp10.Dial(ctx, scheme+"://"+t.address(), options)
	cancel()
	if err != nil {
		return err
	}
	defer connection.Close()
	ctx, cancel = amqpContext(t.readTimeout)
	session, err := connection.NewSession(ctx, nil)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := amqpContext(t.readTimeout)
		defer cancel()
		_ = session.Close(ctx)
	}()
	var outcome sendOutcome
	for _, target := range targets {
		send := func() error {
			ctx, cancel := amqpContext(t.readTimeout)
			defer cancel()
			sender, err := session.NewSender(ctx, target, nil)
			if err != nil {
				return err
			}
			defer func() {
				ctx, cancel := amqpContext(t.readTimeout)
				defer cancel()
				_ = sender.Close(ctx)
			}()
			return sender.Send(ctx, t.message10(body), nil)
		}
		err := send()
		outcome.record(err)
		if err == nil {
			t.markDelivered(target)
		}
	}
	return outcome.err()
}
