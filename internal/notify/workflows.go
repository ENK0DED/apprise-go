package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	workflowsVersionLegacy = "2016-06-01"
	workflowsVersionPA     = "2022-03-01-preview"
	workflowsAdaptiveVer   = "1.4"
)

var workflowsWorkflowRe = regexp.MustCompile(`(?i)^[A-Z0-9_-]+$`)
var workflowsSignatureRe = regexp.MustCompile(`(?i)^[a-z0-9_-]+$`)

type WorkflowsTarget struct {
	host          string
	port          int
	workflowID    string
	signature     string
	includeImage  bool
	powerAutomate bool
	wrap          bool
	apiVersion    string
}

func NewWorkflowsTarget(target *ParsedURL) (*WorkflowsTarget, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	entries := splitPath(target.Path)

	workflowID := strings.TrimSpace(target.Query["workflow"])
	if workflowID == "" {
		workflowID = strings.TrimSpace(target.Query["id"])
	}
	if workflowID == "" && len(entries) > 0 {
		workflowID = entries[0]
		entries = entries[1:]
	}

	signature := strings.TrimSpace(target.Query["signature"])
	if signature == "" {
		signature = strings.TrimSpace(target.Query["sig"])
	}
	if signature == "" && len(entries) > 0 {
		signature = entries[0]
	}

	if workflowID == "" || !workflowsWorkflowRe.MatchString(workflowID) {
		return nil, fmt.Errorf("invalid workflow")
	}
	if signature == "" || !workflowsSignatureRe.MatchString(signature) {
		return nil, fmt.Errorf("invalid signature")
	}

	includeImage := parseBoolWithDefault(target.Query["image"], true)

	powerAutomate := parseBoolWithDefault(target.Query["powerautomate"], false)
	if raw := strings.TrimSpace(target.Query["pa"]); raw != "" {
		powerAutomate = parseBoolWithDefault(raw, powerAutomate)
	}

	wrap := parseBoolWithDefault(target.Query["wrap"], true)

	apiVersion := strings.TrimSpace(target.Query["api-version"])
	if apiVersion == "" {
		apiVersion = strings.TrimSpace(target.Query["ver"])
	}
	if apiVersion == "" {
		if powerAutomate {
			apiVersion = workflowsVersionPA
		} else {
			apiVersion = workflowsVersionLegacy
		}
	}

	return &WorkflowsTarget{
		host:          host,
		port:          target.Port,
		workflowID:    workflowID,
		signature:     signature,
		includeImage:  includeImage,
		powerAutomate: powerAutomate,
		wrap:          wrap,
		apiVersion:    apiVersion,
	}, nil
}

func (w *WorkflowsTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	payload := w.buildPayload(body, title, notifyType)
	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	requestURL := w.buildURL()

	return RequestSpec{
		Method: "POST",
		URL:    requestURL,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}, nil
}

func (w *WorkflowsTarget) Send(body, title string, notifyType NotifyType) error {
	spec, err := w.BuildRequest(body, title, notifyType)
	if err != nil {
		return err
	}

	return SendRequest(spec)
}

func (w *WorkflowsTarget) buildURL() string {
	base := fmt.Sprintf("https://%s", w.host)
	if w.port > 0 {
		base += fmt.Sprintf(":%d", w.port)
	}

	path := ""
	if w.powerAutomate {
		path = "/powerautomate/automations/direct"
	}
	path += fmt.Sprintf("/workflows/%s/triggers/manual/paths/invoke", w.workflowID)

	query := url.Values{}
	query.Set("api-version", w.apiVersion)
	query.Set("sp", "/triggers/manual/run")
	query.Set("sv", "1.0")
	query.Set("sig", w.signature)

	return base + path + "?" + query.Encode()
}

func (w *WorkflowsTarget) buildPayload(body, title string, notifyType NotifyType) map[string]any {
	bodyContent := []map[string]any{}
	if w.includeImage {
		bodyContent = append(bodyContent, map[string]any{
			"type":    "Image",
			"url":     appriseImageURL(notifyType, "32x32"),
			"height":  "32px",
			"altText": string(notifyType),
		})
	}

	if title != "" {
		bodyContent = append(bodyContent, map[string]any{
			"type":   "TextBlock",
			"text":   title,
			"style":  "heading",
			"weight": "Bolder",
			"size":   "Large",
			"id":     "title",
		})
	}

	bodyContent = append(bodyContent, map[string]any{
		"type":  "TextBlock",
		"text":  body,
		"style": "default",
		"wrap":  w.wrap,
		"id":    "body",
	})

	return map[string]any{
		"type": "message",
		"attachments": []map[string]any{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"contentUrl":  nil,
				"content": map[string]any{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": workflowsAdaptiveVer,
					"body":    bodyContent,
					"msteams": map[string]any{
						"width": "full",
					},
				},
			},
		},
	}
}
