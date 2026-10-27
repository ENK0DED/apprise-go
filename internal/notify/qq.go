package notify

import (
	"fmt"
	"net/url"
)

const qqURL = "https://qmsg.zendee.cn/send/%s"

type QQTarget struct {
	token string
}

func NewQQTarget(target *ParsedURL) (*QQTarget, error) {
	token := target.Host
	if rawToken, ok := target.Query["token"]; ok && rawToken != "" {
		token = rawToken
	}
	if token == "" {
		return nil, fmt.Errorf("missing token")
	}

	return &QQTarget{token: token}, nil
}

func (q *QQTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	message := body
	if title != "" {
		message = title + "\n" + body
	}

	values := url.Values{}
	values.Set("msg", message)

	headers := map[string]string{
		"User-Agent":   "Apprise",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded",
	}

	_ = notifyType

	return RequestSpec{
		Method:  "POST",
		URL:     fmt.Sprintf(qqURL, q.token),
		Headers: headers,
		Body:    values.Encode(),
	}, nil
}

func (q *QQTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := q.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func init() {
	RegisterSchemaEntryOrdered(49, SchemaEntry{
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
			"kwargs":    map[string]any{},
			"templates": []any{"{schema}://{token}"},
			"tokens": map[string]any{
				"schema": map[string]any{
					"default":  "qq",
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"qq"},
				},
				"token": map[string]any{
					"map_to":   "token",
					"name":     "User Token",
					"private":  true,
					"regex":    []any{"^[a-z0-9]{24,64}$", "i"},
					"required": true,
					"type":     "string",
				},
			},
		},
		"enabled":   true,
		"protocols": nil,
		"requirements": map[string]any{
			"details":              "",
			"packages_recommended": []any{},
			"packages_required":    []any{},
		},
		"secure_protocols": []string{"qq"},
		"service_name":     "QQ Push",
		"service_url":      "https://github.com/songquanpeng/message-pusher",
		"setup_url":        "https://appriseit.com/services/qq/",
	})
}
