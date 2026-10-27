package parity

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/unraid/apprise-go/internal/notify"
	"github.com/unraid/apprise-go/internal/testutil"
)

type normalizedAMQP struct {
	Body, ContentType, AppID, Expiration string
	DeliveryMode, Priority               uint8
	Timestamp                            int64
	Headers                              map[string]any
}

func TestAMQPBrokerParity(t *testing.T) {
	address := os.Getenv("APPRISE_AMQP_TEST_ADDR")
	if address == "" {
		t.Skip("APPRISE_AMQP_TEST_ADDR selects a dedicated test broker")
	}
	user, password := os.Getenv("APPRISE_AMQP_TEST_USER"), os.Getenv("APPRISE_AMQP_TEST_PASSWORD")
	if user == "" {
		user = "parity"
	}
	if password == "" {
		password = "parity-pass"
	}
	base := (&url.URL{Scheme: "amqp", Host: address, User: url.UserPassword(user, password)}).String()
	connection, err := amqp091.DialConfig(base, amqp091.Config{Dial: amqp091.DefaultDial(4 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	channel, err := connection.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	for _, tc := range []struct {
		name, version, format, options string
		partial, fanout                bool
	}{
		{name: "091-default", version: "0.9.1"},
		{name: "091-properties", version: "v0-9-1", options: "&persistent=no&priority=7&ttl=60000&+service=storage"},
		{name: "091-markdown", version: "0.9", format: "markdown"},
		{name: "091-partial-retry", version: "0.9.1", partial: true},
		{name: "091-fanout", version: "0.9.1", fanout: true},
		{name: "10-default", version: "1.0"},
		{name: "10-properties", version: "v1", options: "&persistent=no&priority=7&ttl=60000&+service=storage"},
		{name: "10-html", version: "1.0", format: "html"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue, err := channel.QueueDeclare("", false, true, true, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _, _ = channel.QueueDelete(queue.Name, false, false, false) }()
			query := "?version=" + url.QueryEscape(tc.version) + "&rto=2" + tc.options
			target := base + "/" + queue.Name
			if strings.Contains(tc.version, "1") && !(strings.Contains(tc.version, "9")) {
				target = base
				query += "&to=" + url.QueryEscape("/queues/"+queue.Name)
			}
			if tc.partial {
				query += "&to=missing-apprise-parity-target&retry=1"
			}
			if tc.fanout {
				exchange := "apprise-parity-" + strconv.FormatInt(time.Now().UnixNano(), 10)
				if err := channel.ExchangeDeclare(exchange, "fanout", false, true, false, false, nil); err != nil {
					t.Fatal(err)
				}
				defer channel.ExchangeDelete(exchange, false, false)
				if err := channel.QueueBind(queue.Name, "", exchange, false, nil); err != nil {
					t.Fatal(err)
				}
				target = base
				query += "&exchange=" + exchange
			}
			target += query
			body := "Body 中文"
			if tc.format == "markdown" {
				body = "**Body**"
			}
			if tc.format == "html" {
				body = "<b>Body</b>"
			}
			args := []string{"--url", target, "--body", body, "--title", "Title"}
			if tc.format != "" {
				args = append(args, "--body-format", tc.format)
			}
			stdout, stderr, err := testutil.RunPythonScript(t, filepath.Join(testutil.RepoRoot(t), "internal/testutil/scripts/capture_amqp.py"), args...)
			if err != nil {
				t.Fatalf("Python AMQP: %v %s", err, stderr)
			}
			var result struct {
				Success bool `json:"success"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if result.Success == tc.partial {
				t.Fatalf("Python success=%v partial=%v, stderr=%s", result.Success, tc.partial, stderr)
			}
			python, ok, err := channel.Get(queue.Name, true)
			if err != nil || !ok {
				t.Fatalf("Python message missing: %v", err)
			}
			if _, ok, err := channel.Get(queue.Name, true); err != nil || ok {
				t.Fatalf("Python duplicated a successful target: %v", err)
			}
			sendErr := notify.SendTargetURL(target, body, "Title", tc.format, notify.NotifyInfo)
			if (sendErr == nil) != result.Success {
				t.Fatalf("success mismatch: Python=%v Go=%v", result.Success, sendErr)
			}
			got, ok, err := channel.Get(queue.Name, true)
			if err != nil || !ok {
				t.Fatalf("Go message missing: %v", err)
			}
			if _, ok, err := channel.Get(queue.Name, true); err != nil || ok {
				t.Fatalf("Go duplicated a successful target: %v", err)
			}
			normalize := func(delivery amqp091.Delivery) normalizedAMQP {
				expiration := delivery.Expiration
				if strings.Contains(tc.options, "ttl=") {
					ttl, err := strconv.Atoi(expiration)
					if err != nil || ttl < 55000 || ttl > 60000 {
						t.Fatalf("invalid broker TTL %q", expiration)
					}
					expiration = "within requested 60000ms"
				}
				headers := map[string]any{}
				if strings.Contains(tc.options, "service=") {
					headers["service"] = delivery.Headers["service"]
					if headers["service"] != "storage" {
						t.Fatalf("missing application header: %v", delivery.Headers)
					}
				}
				return normalizedAMQP{Body: string(delivery.Body), ContentType: delivery.ContentType, AppID: delivery.AppId, Expiration: expiration, DeliveryMode: delivery.DeliveryMode, Priority: delivery.Priority, Timestamp: delivery.Timestamp.Unix(), Headers: headers}
			}
			want, actual := normalize(python), normalize(got)
			if !reflect.DeepEqual(want, actual) {
				t.Fatalf("AMQP mismatch\nPython=%+v\nGo=%+v", want, actual)
			}
		})
	}
}
