package notify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const blueskyDefaultHost = "bsky.social"
const blueskyResolveURL = "https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle"
const blueskyPLCBase = "https://plc.directory"
const blueskyCreateSessionPath = "/xrpc/com.atproto.server.createSession"
const blueskyCreateRecordPath = "/xrpc/com.atproto.repo.createRecord"
const blueskyFixedCreatedAt = "2024-01-01T00:00:00Z"

type BlueskyTarget struct {
	user     string
	host     string
	password string
	did      string
	endpoint string
}

func NewBlueskyTarget(target *ParsedURL) (*BlueskyTarget, error) {
	user := strings.TrimSpace(target.User)
	if user == "" {
		return nil, fmt.Errorf("missing user")
	}

	password := strings.TrimSpace(target.Password)
	if password == "" {
		password = strings.TrimSpace(target.Host)
	}
	if password == "" {
		return nil, fmt.Errorf("missing password")
	}

	host := blueskyDefaultHost
	if strings.Contains(user, ".") {
		parts := strings.SplitN(user, ".", 2)
		if strings.TrimSpace(parts[0]) != "" {
			user = parts[0]
			host = parts[1]
		}
	}

	return &BlueskyTarget{
		user:     user,
		host:     host,
		password: password,
	}, nil
}

func (b *BlueskyTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	_ = title
	_ = notifyType
	return RequestSpec{}, fmt.Errorf("multi-step request")
}

func (b *BlueskyTarget) Send(body, title string, notifyType NotifyType) error {
	if err := b.resolveIdentity(); err != nil {
		return err
	}

	accessToken, err := b.login()
	if err != nil {
		return err
	}

	message := mergeTitleBody(title, body)
	spec, err := b.createRecordSpec(message, accessToken)
	if err != nil {
		return err
	}

	if err := SendRequest(spec); err != nil {
		return err
	}

	_ = notifyType

	return nil
}

func (b *BlueskyTarget) resolveIdentity() error {
	handle := b.user
	if !strings.Contains(handle, ".") {
		handle = handle + "." + b.host
	}

	resolveURL, err := url.Parse(blueskyResolveURL)
	if err != nil {
		return err
	}
	params := resolveURL.Query()
	params.Set("handle", handle)
	resolveURL.RawQuery = params.Encode()

	spec := RequestSpec{
		Method: "GET",
		URL:    resolveURL.String(),
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
		},
	}

	var resolveResponse struct {
		DID string `json:"did"`
	}
	if err := doJSONRequest(spec, &resolveResponse); err != nil {
		return err
	}
	if resolveResponse.DID == "" {
		return fmt.Errorf("missing did")
	}
	b.did = resolveResponse.DID

	if strings.HasPrefix(b.did, "did:plc:") {
		plcURL := blueskyPLCBase + "/" + b.did
		plcSpec := RequestSpec{
			Method: "GET",
			URL:    plcURL,
			Headers: map[string]string{
				"User-Agent":   "Apprise",
				"Content-Type": "application/x-www-form-urlencoded; charset=utf-8",
			},
		}
		var plcResponse struct {
			Service []struct {
				Type            string `json:"type"`
				ServiceEndpoint string `json:"serviceEndpoint"`
			} `json:"service"`
		}
		if err := doJSONRequest(plcSpec, &plcResponse); err != nil {
			return err
		}
		for _, entry := range plcResponse.Service {
			if entry.Type == "AtprotoPersonalDataServer" && entry.ServiceEndpoint != "" {
				b.endpoint = entry.ServiceEndpoint
				break
			}
		}
	}

	if b.endpoint == "" {
		return fmt.Errorf("missing endpoint")
	}

	return nil
}

func (b *BlueskyTarget) login() (string, error) {
	payload := map[string]string{
		"identifier": b.did,
		"password":   b.password,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	spec := RequestSpec{
		Method: "POST",
		URL:    b.endpoint + blueskyCreateSessionPath,
		Headers: map[string]string{
			"User-Agent":   "Apprise",
			"Content-Type": "application/json",
		},
		Body: string(data),
	}

	var response struct {
		AccessJWT string `json:"accessJwt"`
	}
	if err := doJSONRequest(spec, &response); err != nil {
		return "", err
	}
	if response.AccessJWT == "" {
		return "", fmt.Errorf("missing access token")
	}

	return response.AccessJWT, nil
}

func (b *BlueskyTarget) createRecordSpec(message string, accessToken string) (RequestSpec, error) {
	payload := map[string]any{
		"collection": "app.bsky.feed.post",
		"repo":       b.did,
		"record": map[string]any{
			"text":      message,
			"createdAt": blueskyFixedCreatedAt,
			"$type":     "app.bsky.feed.post",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return RequestSpec{}, err
	}

	return RequestSpec{
		Method: "POST",
		URL:    b.endpoint + blueskyCreateRecordPath,
		Headers: map[string]string{
			"User-Agent":    "Apprise",
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + accessToken,
		},
		Body: string(data),
	}, nil
}
