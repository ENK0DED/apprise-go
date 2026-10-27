package notify

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	vapidDefaultTTL           = 0
	vapidJWTExpirationSeconds = 43200
)

var vapidURLByMode = map[string]string{
	"chrome":  "https://fcm.googleapis.com/fcm/send",
	"firefox": "https://updates.push.services.mozilla.com/wpush/v1",
	"edge":    "https://fcm.googleapis.com/fcm/send",
	"opera":   "https://fcm.googleapis.com/fcm/send",
	"apple":   "https://web.push.apple.com/",
}

type VapidTarget struct {
	ready      bool
	readyError error
	deliveryTracker
	store         Store
	retired       map[string]bool
	subscriber    string
	mode          string
	ttl           int
	keyfile       string
	subfile       string
	targets       []string
	privateKey    *ecdsa.PrivateKey
	publicKeyStr  string
	subscriptions map[string]vapidSubscription
}

type vapidSubscription struct {
	endpoint, origin string
	publicKey        *ecdh.PublicKey
	authSecret       []byte
}

func NewVapidTarget(target *ParsedURL) (*VapidTarget, error) {
	subscriber := ""
	if target.User != "" && target.Host != "" {
		subscriber = target.User + "@" + target.Host
	} else if strings.Contains(target.Host, "@") {
		subscriber = target.Host
	}
	if !isSimpleEmail(subscriber) {
		return nil, fmt.Errorf("invalid subscriber")
	}

	mode := strings.ToLower(strings.TrimSpace(target.Query["mode"]))
	if mode == "" {
		mode = "chrome"
	}
	if _, ok := vapidURLByMode[mode]; !ok {
		return nil, fmt.Errorf("invalid mode")
	}

	ttl := vapidDefaultTTL
	if rawTTL := strings.TrimSpace(target.Query["ttl"]); rawTTL != "" {
		if parsed, err := strconv.Atoi(rawTTL); err == nil {
			ttl = parsed
		}
	}

	keyfile := strings.TrimSpace(target.Query["keyfile"])
	subfile := strings.TrimSpace(target.Query["subfile"])

	targets := []string{}
	for _, entry := range splitPath(target.Path) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		targets = append(targets, strings.ToLower(entry))
	}
	if toValue := strings.TrimSpace(target.Query["to"]); toValue != "" {
		for _, entry := range parseDelimitedList(toValue) {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			targets = append(targets, strings.ToLower(entry))
		}
	}
	if len(targets) == 0 {
		targets = append(targets, strings.ToLower(subscriber))
	}

	return &VapidTarget{
		subscriber: subscriber,
		mode:       mode,
		ttl:        ttl,
		keyfile:    keyfile,
		subfile:    subfile,
		targets:    targets,

		retired: map[string]bool{}, store: StoreFor(target),
	}, nil
}

func (v *VapidTarget) BuildRequest(body, title string, notifyType NotifyType) (RequestSpec, error) {
	if err := v.ensureReady(); err != nil {
		return RequestSpec{}, err
	}
	if len(v.targets) == 0 {
		return RequestSpec{}, fmt.Errorf("missing targets")
	}
	payload, err := v.buildPayload(body, v.targets[0])
	if err != nil {
		return RequestSpec{}, err
	}
	entry, ok := v.subscriptions[strings.ToLower(v.targets[0])]
	if !ok {
		return RequestSpec{}, fmt.Errorf("missing subscription")
	}
	headers, err := v.buildHeadersForOrigin(entry.origin)
	if err != nil {
		return RequestSpec{}, err
	}
	return RequestSpec{
		Method:  "POST",
		URL:     entry.endpoint,
		Headers: headers,
		Body:    string(payload),
	}, nil
}

func (v *VapidTarget) Send(body, title string, notifyType NotifyType) error {
	if err := v.ensureReady(); err != nil {
		return err
	}
	var outcome sendOutcome
	if v.retired == nil {
		v.retired = map[string]bool{}
	}
	if v.store != nil {
		var retired []string
		if storeGetJSON(v.store, "retired", &retired) {
			for _, endpoint := range retired {
				v.retired[endpoint] = true
			}
		}
	}
	delivered := 0
	expired := map[string]bool{}
	for _, target := range v.targets {
		if v.isDelivered(target) {
			delivered++
			continue
		}
		entry, ok := v.subscriptions[strings.ToLower(target)]
		if !ok {
			outcome.record(fmt.Errorf("missing Vapid subscription: %s", target))
			continue
		}
		if v.retired[entry.endpoint] {
			continue
		}
		payload, err := v.buildPayload(body, target)
		if err != nil {
			outcome.record(err)
			continue
		}
		headers, err := v.buildHeadersForOrigin(entry.origin)
		if err != nil {
			outcome.record(err)
			continue
		}
		req, err := (RequestSpec{Method: http.MethodPost, URL: entry.endpoint, Headers: headers, Body: string(payload)}).HTTPRequest()
		if err != nil {
			outcome.record(err)
			continue
		}
		resp, err := httpClient().Do(req)
		if err != nil {
			outcome.record(err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK, http.StatusNoContent, http.StatusCreated, http.StatusAccepted:
			v.markDelivered(target)
			delivered++
		case http.StatusNotFound, http.StatusGone:
			expired[target] = true
			v.retired[entry.endpoint] = true
			outcome.record(&HTTPStatusError{StatusCode: resp.StatusCode})
		default:
			outcome.record(&HTTPStatusError{StatusCode: resp.StatusCode})
		}
	}
	if len(expired) > 0 {
		v.pruneSubscriptions(expired)
		if v.store != nil {
			var endpoints []string
			for endpoint := range v.retired {
				endpoints = append(endpoints, endpoint)
			}
			_ = v.store.Set("retired", endpoints, 30*24*time.Hour)
		}
	}
	if delivered == 0 {
		outcome.record(fmt.Errorf("no Vapid subscriptions could be notified"))
	}
	return outcome.err()
}

// Prune only named expired entries, preserving the rest of the source document.
// A read-only source stays untouched; retired endpoints remain suppressed here.
func (v *VapidTarget) pruneSubscriptions(expired map[string]bool) {
	if v.subfile == "" {
		return
	}
	info, err := os.Lstat(v.subfile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0200 == 0 {
		return
	}
	data, err := os.ReadFile(v.subfile)
	if err != nil {
		return
	}
	var document map[string]json.RawMessage
	if json.Unmarshal(data, &document) != nil {
		return
	}
	if _, single := document["endpoint"]; single {
		document = map[string]json.RawMessage{}
	} else {
		for name := range document {
			if expired[strings.ToLower(name)] {
				delete(document, name)
			}
		}
	}
	data, err = json.MarshalIndent(document, "", "  ")
	if err != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(v.subfile), ".apprise-vapid-*")
	if err != nil {
		return
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(info.Mode().Perm()); err == nil {
		_, err = file.Write(append(data, '\n'))
	}
	err = errors.Join(err, file.Close())
	if err == nil {
		_ = os.Rename(name, v.subfile)
	}
}

func (v *VapidTarget) buildHeadersForOrigin(origin string) (map[string]string, error) {
	jwtToken := strings.TrimSpace(os.Getenv("APPRISE_VAPID_TEST_JWT"))
	publicKey := strings.TrimSpace(os.Getenv("APPRISE_VAPID_TEST_PUBLIC_KEY"))
	if jwtToken == "" {
		token, err := buildVapidJWT(v.privateKey, v.subscriber, origin)
		if err != nil {
			return nil, err
		}
		jwtToken = token
	}
	if publicKey == "" {
		publicKey = v.publicKeyStr
	}

	headers := map[string]string{
		"User-Agent":       "Apprise",
		"TTL":              strconv.Itoa(v.ttl),
		"Content-Encoding": "aes128gcm",
		"Content-Type":     "application/octet-stream",
		"Authorization": fmt.Sprintf(
			"vapid t=%s, k=%s",
			jwtToken,
			publicKey,
		),
	}
	return headers, nil
}

func (v *VapidTarget) buildPayload(body, target string) ([]byte, error) {
	override := strings.TrimSpace(os.Getenv("APPRISE_VAPID_TEST_ENCRYPTED"))
	if override != "" {
		data, err := base64.StdEncoding.DecodeString(override)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	subscription, ok := v.subscriptions[strings.ToLower(target)]
	if !ok {
		return nil, fmt.Errorf("missing subscription")
	}

	return encryptWebPush([]byte(body), subscription.publicKey, subscription.authSecret)
}

func loadVapidKey(path string) (*ecdsa.PrivateKey, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, "", fmt.Errorf("invalid pem")
	}

	var key *ecdsa.PrivateKey
	if parsed, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		key = parsed
	} else if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if ecKey, ok := parsed.(*ecdsa.PrivateKey); ok {
			key = ecKey
		}
	}
	if key == nil {
		return nil, "", fmt.Errorf("invalid key")
	}

	ecdhPub, err := key.PublicKey.ECDH()
	if err != nil {
		return nil, "", err
	}
	publicKeyStr := base64.RawURLEncoding.EncodeToString(ecdhPub.Bytes())
	return key, publicKeyStr, nil
}

type vapidSubscriptionFile struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func loadVapidSubscriptions(path, defaultName string) (map[string]vapidSubscription, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var mapped map[string]vapidSubscriptionFile
	if err := json.Unmarshal(data, &mapped); err == nil && len(mapped) > 0 {
		return parseVapidSubscriptions(mapped)
	}

	var single vapidSubscriptionFile
	if err := json.Unmarshal(data, &single); err != nil {
		return nil, err
	}
	if defaultName == "" {
		defaultName = "default"
	}
	return parseVapidSubscriptions(map[string]vapidSubscriptionFile{
		strings.ToLower(defaultName): single,
	})
}

func parseVapidSubscriptions(input map[string]vapidSubscriptionFile) (map[string]vapidSubscription, error) {
	subscriptions := make(map[string]vapidSubscription, len(input))
	for name, entry := range input {
		origin, err := vapidOrigin(entry.Endpoint)
		if err != nil {
			continue
		}
		publicKeyBytes, err := decodeBase64URL(entry.Keys.P256dh)
		if err != nil {
			return nil, err
		}
		publicKey, err := ecdh.P256().NewPublicKey(publicKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid public key")
		}

		authSecret, err := decodeBase64URL(entry.Keys.Auth)
		if err != nil {
			return nil, err
		}

		subscriptions[strings.ToLower(name)] = vapidSubscription{
			endpoint: entry.Endpoint, origin: origin,
			publicKey:  publicKey,
			authSecret: authSecret,
		}
	}
	return subscriptions, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("missing base64 value")
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(value)
}

func buildVapidJWT(privateKey *ecdsa.PrivateKey, subscriber, audience string) (string, error) {
	header := struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}{
		Alg: "ES256",
		Typ: "JWT",
	}

	payload := struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}{
		Aud: audience,
		Exp: fixedTime().Unix() + vapidJWTExpirationSeconds,
		Sub: "mailto:" + subscriber,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	signature, err := signVapid(privateKey, signingInput)
	if err != nil {
		return "", err
	}
	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)
	return signingInput + "." + signatureB64, nil
}

func signVapid(privateKey *ecdsa.PrivateKey, input string) ([]byte, error) {
	hash := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return nil, err
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()

	signature := make([]byte, 64)
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):], sBytes)
	return signature, nil
}

func encryptWebPush(message []byte, publicKey *ecdh.PublicKey, authSecret []byte) ([]byte, error) {
	ephemeralKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	ephemeralPub := ephemeralKey.PublicKey().Bytes()

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	// ECDH returns the shared secret as the fixed-length (32-byte) X coordinate.
	sharedSecret, err := ephemeralKey.ECDH(publicKey)
	if err != nil {
		return nil, err
	}

	recipientPub := publicKey.Bytes()
	info := append([]byte("WebPush: info\x00"), recipientPub...)
	info = append(info, ephemeralPub...)

	hkdfSecret := hkdf.New(sha256.New, sharedSecret, authSecret, info)
	secretKey := make([]byte, 32)
	if _, err := hkdfSecret.Read(secretKey); err != nil {
		return nil, err
	}

	hkdfKey := hkdf.New(sha256.New, secretKey, salt, []byte("Content-Encoding: aes128gcm\x00"))
	contentKey := make([]byte, 16)
	if _, err := hkdfKey.Read(contentKey); err != nil {
		return nil, err
	}

	hkdfNonce := hkdf.New(sha256.New, secretKey, salt, []byte("Content-Encoding: nonce\x00"))
	nonce := make([]byte, 12)
	if _, err := hkdfNonce.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext, err := encryptAES128GCM(contentKey, nonce, append(message, 0x02))
	if err != nil {
		return nil, err
	}

	header := make([]byte, 0, 16+4+1+len(ephemeralPub)+len(ciphertext))
	header = append(header, salt...)
	header = append(header, 0x00, 0x00, 0x10, 0x00)
	header = append(header, byte(len(ephemeralPub)))
	header = append(header, ephemeralPub...)
	header = append(header, ciphertext...)
	return header, nil
}

func encryptAES128GCM(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aesBlock(key)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

func aesBlock(key []byte) (cipher.Block, error) {
	return aes.NewCipher(key)
}

func newGCM(block cipher.Block) (cipher.AEAD, error) {
	return cipher.NewGCM(block)
}

func init() {
	RegisterSchemaEntryOrdered(23, SchemaEntry{
		"attachment_support": false,
		"category":           "native",
		"details": map[string]any{
			"args": map[string]any{
				"from": map[string]any{
					"alias_of": "subscriber",
				},
				"image": map[string]any{
					"default":  true,
					"map_to":   "include_image",
					"name":     "Include Image",
					"private":  false,
					"required": false,
					"type":     "bool",
				},
				"keyfile": map[string]any{
					"map_to":   "keyfile",
					"name":     "PEM Private KeyFile",
					"private":  true,
					"required": false,
					"type":     "string",
				},
				"mode": map[string]any{
					"default":  "chrome",
					"map_to":   "mode",
					"name":     "Mode",
					"private":  false,
					"required": false,
					"type":     "choice:string",
					"values":   []any{"chrome", "firefox", "edge", "opera", "apple"},
				},
				"subfile": map[string]any{
					"map_to":   "subfile",
					"name":     "Subscripion File",
					"private":  true,
					"required": false,
					"type":     "string",
				},
				"to": map[string]any{
					"alias_of": "targets",
					"delim":    []any{",", " "},
				},
				"ttl": map[string]any{
					"default":  0,
					"map_to":   "ttl",
					"max":      60,
					"min":      0,
					"name":     "ttl",
					"private":  false,
					"required": false,
					"type":     "int",
				},
			},
			"kwargs":    map[string]any{},
			"templates": []any{"{schema}://{subscriber}", "{schema}://{subscriber}/{targets}"},
			"tokens": map[string]any{
				"schema": map[string]any{
					"default":  "vapid",
					"map_to":   "schema",
					"name":     "Schema",
					"private":  false,
					"required": true,
					"type":     "choice:string",
					"values":   []any{"vapid"},
				},
				"subscriber": map[string]any{
					"map_to":   "subscriber",
					"name":     "API Key",
					"private":  true,
					"required": true,
					"type":     "string",
				},
				"targets": map[string]any{
					"delim":    []any{"/"},
					"group":    []any{},
					"map_to":   "targets",
					"name":     "Targets",
					"private":  false,
					"required": false,
					"type":     "list:string",
				},
			},
		},
		"enabled":   true,
		"protocols": nil,
		"requirements": map[string]any{
			"details":              "",
			"packages_recommended": []any{},
			"packages_required":    []string{"cryptography"},
		},
		"secure_protocols": []string{"vapid"},
		"service_name":     "Vapid Web Push Notifications",
		"service_url":      "https://datatracker.ietf.org/doc/html/draft-thomson-webpush-vapid",
		"setup_url":        "https://appriseit.com/services/vapid/",
	})
}

func vapidOrigin(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("invalid Vapid HTTPS endpoint")
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" && port != "443" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("invalid Vapid endpoint port")
		}
		host += ":" + port
	}
	return "https://" + host, nil
}

func (v *VapidTarget) ensureReady() error {
	if v.ready {
		return v.readyError
	}
	v.ready = true
	if v.keyfile != "" && os.Getenv("APPRISE_VAPID_TEST_JWT") == "" {
		v.privateKey, v.publicKeyStr, v.readyError = loadVapidKey(v.keyfile)
		if v.readyError != nil {
			return v.readyError
		}
	}
	if public := os.Getenv("APPRISE_VAPID_TEST_PUBLIC_KEY"); public != "" {
		v.publicKeyStr = public
	}
	if v.subfile != "" {
		v.subscriptions, v.readyError = loadVapidSubscriptions(v.subfile, strings.ToLower(v.subscriber))
	}
	return v.readyError
}
