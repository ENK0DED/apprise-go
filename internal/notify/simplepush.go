package notify

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const simplepushURL = "https://api.simplepu.sh/v1/tasks/json"

type simplePushRecipient struct{ kind, target string }
type simplePushKey struct {
	key         []byte
	fingerprint string
}

type SimplePushTarget struct {
	deliveryTracker
	parsed                                      *ParsedURL
	token, credential, password, tag, topicAuth string
	seed                                        []byte
	integration, broadcast, shared              bool
	priority                                    *int
	volume                                      *float64
	links                                       []string
	recipients                                  []simplePushRecipient
	keys                                        map[string]simplePushKey
	accountSalt                                 string
	orgKeys                                     map[int][]byte
}

func NewSimplePushTarget(p *ParsedURL) (*SimplePushTarget, error) {
	t := &SimplePushTarget{parsed: p, token: p.Host, password: p.Password, tag: strings.TrimSpace(p.Query["sptag"]), topicAuth: strings.TrimSpace(p.Query["topic_auth_token"]), broadcast: parseBoolWithDefault(p.Query["broadcast"], false), shared: parseBoolWithDefault(p.Query["shared"], false), links: strings.Fields(p.Query["links"]), keys: make(map[string]simplePushKey)}
	if fields := strings.Fields(t.token); len(fields) > 0 {
		t.token = fields[0]
	} else {
		return nil, fmt.Errorf("missing Simplepush token")
	}
	t.credential = t.token
	if p.User != "" {
		t.password = p.User
	}
	if p.Query["password"] != "" {
		t.password = p.Query["password"]
	}
	t.integration = strings.HasPrefix(t.token, "spi_")
	if t.integration {
		credential, seed, found := strings.Cut(t.token, ".")
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(seed, "="))
		if !found || credential == "spi_" || err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("invalid Simplepush integration token")
		}
		t.credential = credential
		t.seed = decoded
	}
	entries := append(splitPath(p.Path), sortedUniqueTargets(parseDelimitedList(p.Query["to"]))...)
	seen := map[string]bool{}
	var topics, members []simplePushRecipient
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" || seen[entry] {
			continue
		}
		seen[entry] = true
		if strings.HasPrefix(entry, "@") {
			member := strings.TrimSpace(entry[1:])
			if member == "" {
				return nil, fmt.Errorf("invalid Simplepush member")
			}
			members = append(members, simplePushRecipient{"member", member})
		} else {
			topics = append(topics, simplePushRecipient{"topic", entry})
		}
	}
	t.recipients = append(topics, members...)
	if !t.integration && (len(members) > 0 || t.broadcast) {
		return nil, fmt.Errorf("simplepush members and broadcast require integration token")
	}
	if t.integration && ((t.broadcast && len(t.recipients) > 0) || (!t.broadcast && len(t.recipients) == 0) || t.password != "") {
		return nil, fmt.Errorf("invalid Simplepush organization routing or password")
	}
	if len(t.recipients) == 0 {
		t.recipients = []simplePushRecipient{{}}
	}
	if value := strings.ToLower(strings.TrimSpace(p.Query["priority"])); value != "" {
		priority, err := strconv.Atoi(value)
		if err != nil || value != strconv.Itoa(priority) || priority < 1 || priority > 5 {
			priority = 0
			for _, choice := range []struct {
				prefix string
				value  int
			}{{"mi", 1}, {"l", 2}, {"d", 3}, {"n", 3}, {"h", 4}, {"c", 5}, {"e", 5}, {"ma", 5}} {
				if strings.HasPrefix(value, choice.prefix) {
					priority = choice.value
					break
				}
			}
		}
		if priority == 0 {
			return nil, fmt.Errorf("invalid Simplepush priority")
		}
		t.priority = &priority
	}
	if value := p.Query["critical_volume"]; value != "" {
		volume, err := strconv.ParseFloat(value, 64)
		if err != nil || !(volume > 0 && volume <= 1) || t.priority == nil || *t.priority != 5 {
			return nil, fmt.Errorf("invalid Simplepush critical volume")
		}
		t.volume = &volume
	}
	if len(t.links) > 25 {
		return nil, fmt.Errorf("simplepush supports at most 25 links")
	}
	return t, nil
}

func (t *SimplePushTarget) authHeaders() map[string]string {
	headers := map[string]string{"User-Agent": "Apprise"}
	if t.integration {
		headers["Authorization"] = "Bearer " + t.credential
	} else {
		headers["API-Token"] = t.credential
	}
	return headers
}

func (t *SimplePushTarget) getJSON(endpoint string) (map[string]any, error) {
	headers := t.authHeaders()
	headers["Accept"] = "application/json"
	req, err := (RequestSpec{Method: http.MethodGet, URL: endpoint, Headers: headers}).HTTPRequest()
	if err != nil {
		return nil, err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, &HTTPStatusError{StatusCode: response.StatusCode}
	}
	var content map[string]any
	if err := json.NewDecoder(response.Body).Decode(&content); err != nil || content == nil {
		return nil, fmt.Errorf("simplepush returned unreadable metadata")
	}
	return content, nil
}

func (t *SimplePushTarget) encryption(recipient simplePushRecipient) ([]byte, map[string]any, error) {
	if t.integration {
		if t.orgKeys == nil {
			content, err := t.getJSON("https://api.simplepu.sh/v1/org/integration/keys")
			if err != nil {
				return nil, nil, err
			}
			scopes, _ := content["scopes"].([]any)
			allowed := false
			for _, scope := range scopes {
				if scope == "send" {
					allowed = true
				}
			}
			if !allowed {
				return nil, nil, fmt.Errorf("simplepush integration lacks send scope")
			}
			keys := map[int][]byte{}
			if jsonValueTruthy(content["enabled"]) {
				wraps, _ := content["wrappedKeys"].([]any)
				if len(wraps) == 0 {
					return nil, nil, fmt.Errorf("simplepush encrypted organization has no wrapped keys")
				}
				publicText, _ := content["adminPubkeyB64"].(string)
				public, err := base64.StdEncoding.DecodeString(publicText)
				if err != nil {
					return nil, nil, err
				}
				for _, raw := range wraps {
					wrap, ok := raw.(map[string]any)
					if !ok {
						return nil, nil, fmt.Errorf("invalid Simplepush wrapped key")
					}
					version, err := strconv.Atoi(fmt.Sprint(wrap["version"]))
					if err != nil {
						return nil, nil, err
					}
					text, _ := wrap["blob"].(string)
					blob, err := base64.StdEncoding.DecodeString(text)
					if err != nil {
						return nil, nil, err
					}
					key, err := openSimplePushOrganizationKey(t.seed, public, blob)
					if err != nil {
						return nil, nil, err
					}
					keys[version] = key
				}
			}
			t.orgKeys = keys
		}
		if len(t.orgKeys) == 0 {
			return nil, nil, nil
		}
		newest := 0
		first := true
		for version := range t.orgKeys {
			if first || version > newest {
				newest = version
				first = false
			}
		}
		return t.orgKeys[newest], map[string]any{"type": "org", "v": newest}, nil
	}
	if t.password == "" {
		return nil, nil, nil
	}
	salt := recipient.target
	if recipient.kind != "topic" {
		if t.accountSalt == "" {
			content, err := t.getJSON("https://api.simplepu.sh/v1/user")
			if err != nil {
				return nil, nil, err
			}
			t.accountSalt, _ = content["passwordSalt"].(string)
			if t.accountSalt == "" {
				return nil, nil, fmt.Errorf("simplepush returned no password salt")
			}
		}
		salt = t.accountSalt
	}
	key, found := t.keys[salt]
	if !found {
		key.key, key.fingerprint = deriveSimplePushKey(t.password, salt)
		t.keys[salt] = key
	}
	return key.key, map[string]any{"type": "personal", "keyFingerprint": key.fingerprint}, nil
}

func (t *SimplePushTarget) prepare(body, title string, recipient simplePushRecipient, attachments []Attachment) (RequestSpec, [][]byte, error) {
	payload := map[string]any{}
	if recipient.kind != "" {
		payload[recipient.kind] = recipient.target
	}
	if t.broadcast {
		payload["broadcast"] = true
	}
	if t.shared {
		payload["shared"] = true
	}
	if recipient.kind == "topic" && t.topicAuth != "" {
		payload["topicAuthToken"] = t.topicAuth
	}
	if t.priority != nil {
		payload["priority"] = *t.priority
	}
	if t.volume != nil {
		payload["criticalVolume"] = *t.volume
	}
	if body != "" && targetNotifyFormat(t.parsed) == "markdown" {
		payload["contentFormat"] = "markdown"
	}
	content := body
	if content == "" {
		content = title
	}
	if content == "" {
		var names []string
		for i, a := range attachments {
			name := a.Name
			if name == "" {
				name = fmt.Sprintf("file%03d.dat", i+1)
			}
			names = append(names, name)
		}
		content = strings.Join(names, ", ")
	}
	fields := map[string]string{"content": content, "tag": t.tag}
	if body != "" {
		fields["title"] = title
	}
	links := append([]string(nil), t.links...)
	blobs := make([][]byte, len(attachments))
	for i, a := range attachments {
		blobs[i] = a.Data
	}
	key, marker, err := t.encryption(recipient)
	if err != nil {
		return RequestSpec{}, nil, err
	}
	encrypt := func(value string) (string, error) {
		sealed, err := sealSimplePush([]byte(value), key)
		return base64.StdEncoding.EncodeToString(sealed), err
	}
	if len(key) > 0 {
		for _, name := range []string{"title", "content", "tag"} {
			if fields[name] == "" {
				continue
			}
			value, err := encrypt(fields[name])
			if err != nil {
				return RequestSpec{}, nil, err
			}
			fields[name] = value
		}
		if len(fields["title"]) > 500 || len(fields["content"]) > 10000 {
			return RequestSpec{}, nil, fmt.Errorf("simplepush encrypted text exceeds field limit")
		}
		for i, link := range links {
			value, err := encrypt(link)
			if err != nil {
				return RequestSpec{}, nil, err
			}
			links[i] = value
		}
		for i, blob := range blobs {
			value, err := sealSimplePush(blob, key)
			if err != nil {
				return RequestSpec{}, nil, err
			}
			blobs[i] = value
		}
		payload["encryption"] = marker
	}
	for name, value := range fields {
		if value != "" {
			payload[name] = value
		}
	}
	if len(links) > 0 {
		payload["links"] = links
	}
	if len(blobs) > 0 {
		files := make([]map[string]any, len(blobs))
		for i, blob := range blobs {
			a := attachments[i]
			name := a.Name
			if name == "" {
				name = fmt.Sprintf("file%03d.dat", i+1)
			}
			mime := a.MIMEType
			if mime == "" {
				mime = "application/octet-stream"
			}
			hash := sha256.Sum256(blob)
			files[i] = map[string]any{"filename": name, "contentType": mime, "size": len(blob), "checksumSha256": base64.StdEncoding.EncodeToString(hash[:])}
		}
		payload["files"] = files
	}
	data, err := json.Marshal(payload)
	headers := t.authHeaders()
	headers["Content-Type"] = "application/json"
	return RequestSpec{Method: http.MethodPost, URL: simplepushURL, Headers: headers, Body: string(data)}, blobs, err
}

func (t *SimplePushTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	spec, _, err := t.prepare(body, title, t.recipients[0], nil)
	return spec, err
}
func (t *SimplePushTarget) Send(body, title string, kind NotifyType) error {
	return t.SendWithAttachments(body, title, kind, nil)
}
func (t *SimplePushTarget) SendWithAttachments(body, title string, _ NotifyType, attachments []Attachment) error {
	var outcome sendOutcome
	for _, recipient := range t.recipients {
		deliveryKey := recipient.kind + ":" + recipient.target
		if t.isDelivered(deliveryKey) {
			continue
		}
		spec, blobs, err := t.prepare(body, title, recipient, attachments)
		var response map[string]any
		if err == nil {
			response, err = simplePushTask(spec)
		}
		if err != nil {
			outcome.record(err)
			continue
		}
		t.markDelivered(deliveryKey)
		created, _ := response["attachments"].([]any)
		for i, raw := range created {
			if i >= len(blobs) {
				break
			}
			a, _ := raw.(map[string]any)
			id, _ := a["id"].(string)
			outcome.record(t.upload(id, blobs[i]))
		}
	}
	return outcome.err()
}

func simplePushTask(spec RequestSpec) (map[string]any, error) {
	req, err := spec.HTTPRequest()
	if err != nil {
		return nil, err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 && response.StatusCode != 201 {
		return nil, &HTTPStatusError{StatusCode: response.StatusCode}
	}
	var content map[string]any
	_ = json.NewDecoder(response.Body).Decode(&content)
	return content, nil
}

func (t *SimplePushTarget) upload(id string, blob []byte) error {
	if id == "" {
		return fmt.Errorf("simplepush returned no attachment ID")
	}
	step := func(name string) ([]byte, error) {
		req, err := (RequestSpec{Method: http.MethodPost, URL: "https://api.simplepu.sh/v1/attachments/" + id + "/" + name, Headers: t.authHeaders()}).HTTPRequest()
		if err != nil {
			return nil, err
		}
		response, err := httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 && response.StatusCode != 201 && response.StatusCode != 204 {
			return nil, &HTTPStatusError{StatusCode: response.StatusCode}
		}
		return io.ReadAll(response.Body)
	}
	perform := func() error {
		data, err := step("upload-url")
		if err != nil {
			return err
		}
		var content struct {
			URL string `json:"presignedPutUrl"`
		}
		if json.Unmarshal(data, &content) != nil || content.URL == "" {
			return fmt.Errorf("simplepush returned no attachment upload URL")
		}
		req, err := (RequestSpec{Method: http.MethodPut, URL: content.URL, Headers: map[string]string{"Content-Type": "application/octet-stream"}, Body: string(blob)}).HTTPRequest()
		if err != nil {
			return err
		}
		response, err := httpClient().Do(req)
		if err != nil {
			return err
		}
		response.Body.Close()
		if response.StatusCode != 200 && response.StatusCode != 201 && response.StatusCode != 204 {
			return &HTTPStatusError{StatusCode: response.StatusCode}
		}
		_, err = step("complete")
		return err
	}
	if err := perform(); err != nil {
		_, _ = step("failed")
		return err
	}
	return nil
}
