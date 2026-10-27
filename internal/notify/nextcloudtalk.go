package notify

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const nextcloudTalkDefaultMessage = "Apprise"

type NextcloudTalkTarget struct {
	deliveryTracker
	host      string
	port      int
	secure    bool
	user      string
	password  string
	targets   []string
	urlPrefix string
	headers   map[string]string
	secret    string
	silent    bool
}

func NewNextcloudTalkTarget(target *ParsedURL) (*NextcloudTalkTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	user := strings.TrimSpace(target.User)
	if target.Query["secret"] == "" && (user == "" || strings.TrimSpace(target.Password) == "") {
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
		headers:   headers, secret: target.Query["secret"], silent: parseBoolWithDefault(target.Query["silent"], false),
	}, nil
}

func (n *NextcloudTalkTarget) BuildRequest(body, title string, kind NotifyType) (RequestSpec, error) {
	if len(n.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}
	return n.requestFor(n.targets[0], body, title)
}
func (n *NextcloudTalkTarget) requestFor(target, body, title string) (RequestSpec, error) {
	message := n.message(body, title)
	payload := map[string]any{"message": message}
	if n.silent {
		payload["silent"] = true
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}
	headers := n.buildHeaders()
	if n.secret != "" {
		nonce, signature, err := nextcloudTalkSignature(n.secret, message)
		if err != nil {
			return RequestSpec{}, err
		}
		headers["X-Nextcloud-Talk-Bot-Random"] = nonce
		headers["X-Nextcloud-Talk-Bot-Signature"] = signature
	}
	return RequestSpec{Method: "POST", URL: n.buildURL(target), Headers: headers, Body: string(data)}, nil
}
func (n *NextcloudTalkTarget) Send(body, title string, kind NotifyType) error {
	if len(n.targets) == 0 {
		return fmt.Errorf("missing targets")
	}
	var outcome sendOutcome
	for _, target := range n.targets {
		if n.isDelivered(target) {
			continue
		}
		spec, err := n.requestFor(target, body, title)
		if err == nil {
			err = SendRequest(spec)
		}
		outcome.record(err)
		if err == nil {
			n.markDelivered(target)
		}
	}
	return outcome.err()
}
func nextcloudTalkSignature(secret, message string) (string, string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", "", err
	}
	nonce := hex.EncodeToString(random)
	return nonce, nextcloudTalkMAC(secret, nonce, message), nil
}
func nextcloudTalkMAC(secret, nonce, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(nonce + message))
	return hex.EncodeToString(mac.Sum(nil))
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
	if n.secret == "" {
		headers["Authorization"] = basicAuthHeader(n.user, n.password)
	}
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

	prefix := ""
	if n.urlPrefix != "" {
		prefix = "/" + n.urlPrefix
	}
	endpoint := "chat/" + target
	if n.secret != "" {
		endpoint = "bot/" + target + "/message"
	}
	path := prefix + "/ocs/v2.php/apps/spreed/api/v1/" + endpoint
	return base + path
}

func init() {
	RegisterSchemaEntryOrdered(27, SchemaEntry{
		"attachment_support": false,
		"category":           "native",
		"details": map[string]any{
			"args": map[string]any{
				"cto": map[string]any{
					"default":  4,
					"map_to":   "cto",
					"name":     "Socket Connect Timeout",
					"private":  false,
					"required": false,
					"type":     "float",
				},
				"emojis": map[string]any{
					"default":  false,
					"map_to":   "emojis",
					"name":     "Interpret Emojis",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"format": map[string]any{
					"default":   "text",
					"map_to":    "format",
					"name":      "Notify Format",
					"private":   false,
					"required":  false,
					"supported": []any{"text"},
					"type":      "choice:string",
					"values":    []any{"html", "markdown", "text"},
				},
				"optional": map[string]any{
					"default":  false,
					"map_to":   "optional",
					"name":     "Optional Service",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"overflow": map[string]any{
					"default":  "upstream",
					"map_to":   "overflow",
					"name":     "Overflow Mode",
					"private":  false,
					"required": false,
					"type":     "choice:string",
					"values":   []any{"split", "truncate", "upstream"},
				},
				"redirect": map[string]any{
					"default":  true,
					"map_to":   "redirect",
					"name":     "Follow Redirects",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"retry": map[string]any{
					"default":  0,
					"map_to":   "retry",
					"max":      10,
					"min":      0,
					"name":     "Service Retry",
					"private":  false,
					"required": false,
					"type":     "int",
				},
				"rto": map[string]any{
					"default":  4,
					"map_to":   "rto",
					"name":     "Socket Read Timeout",
					"private":  false,
					"required": false,
					"type":     "float",
				},
				"secret": map[string]any{
					"alias_of": "secret",
				},
				"silent": map[string]any{
					"default":  false,
					"map_to":   "silent",
					"name":     "Silent Notification",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"store": map[string]any{
					"default":  true,
					"map_to":   "store",
					"name":     "Persistent Storage",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"tz": map[string]any{
					"default":  nil,
					"map_to":   "tz",
					"name":     "Timezone",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"url_prefix": map[string]any{
					"map_to":   "url_prefix",
					"name":     "URL Prefix",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"verify": map[string]any{
					"default":  true,
					"map_to":   "verify",
					"name":     "Verify SSL",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"wait": map[string]any{
					"default":  0,
					"map_to":   "wait",
					"max":      20,
					"min":      0,
					"name":     "Inter-Retry Wait",
					"private":  false,
					"required": false,
					"type":     "float",
				},
			},
			"kwargs": map[string]any{
				"headers": map[string]any{
					"map_to":   "headers",
					"name":     "HTTP Header",
					"prefix":   "+",
					"private":  false,
					"required": false,
					"type":     "string",
				},
			},
			"templates": []any{"{schema}://{user}:{password}@{host}/{targets}", "{schema}://{user}:{password}@{host}:{port}/{targets}", "{schema}://{host}/{targets}?secret={secret}", "{schema}://{host}:{port}/{targets}?secret={secret}"},
			"tokens": map[string]any{
				"host": map[string]any{
					"map_to":   "host",
					"name":     "Hostname",
					"private":  false,
					"required": true,
					"type":     "string",
				},
				"password": map[string]any{
					"map_to":   "password",
					"name":     "Password",
					"private":  true,
					"required": true,
					"type":     "string",
				},
				"port": map[string]any{
					"map_to":   "port",
					"max":      65535,
					"min":      1,
					"name":     "Port",
					"private":  false,
					"required": false,
					"type":     "int",
				},
				"schema": map[string]any{
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"nctalk", "nctalks"},
				},
				"secret": map[string]any{
					"map_to":   "secret",
					"name":     "Secret",
					"private":  true,
					"required": true,
					"type":     "string",
				},
				"target_room_id": map[string]any{
					"map_to":   "targets",
					"name":     "Room ID",
					"private":  false,
					"required": false,
					"type":     "string",
				},
				"targets": map[string]any{
					"delim":    []any{"/"},
					"group":    []any{"target_room_id"},
					"map_to":   "targets",
					"name":     "Targets",
					"private":  false,
					"required": true,
					"type":     "list:string",
				},
				"user": map[string]any{
					"map_to":   "user",
					"name":     "Username",
					"private":  false,
					"required": true,
					"type":     "string",
				},
			},
		},
		"enabled":   true,
		"protocols": []string{"nctalk"},
		"requirements": map[string]any{
			"details":              "",
			"packages_recommended": []any{},
			"packages_required":    []any{},
		},
		"secure_protocols": []string{"nctalks"},
		"service_name":     "Nextcloud Talk",
		"service_url":      "https://nextcloud.com/talk",
		"setup_url":        "https://appriseit.com/services/nextcloudtalk/",
	})
}
