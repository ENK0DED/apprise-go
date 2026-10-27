package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/unraid/apprise-go/internal/matrixolm"
)

const matrixSASFilter = `{"presence": {"types": []}, "account_data": {"types": []}, "room": {"rooms": []}}`
const matrixSASEvent = "m.key.verification."

func matrixSASID(value string) bool { return value != "" && utf8.RuneCountInString(value) <= 512 }
func matrixSASString(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return value
}
func matrixSASMap(value any) map[string]any { result, _ := value.(map[string]any); return result }
func matrixSASHas(values any, want string) bool {
	list, _ := values.([]any)
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
func (m *MatrixTarget) autoVerifyBinding() string {
	var binding string
	storeGetJSON(m.store, "e2ee_device_binding", &binding)
	return binding
}
func (m *MatrixTarget) autoVerify() bool {
	if !m.autoVerifyMu.TryLock() {
		return false
	}
	defer m.autoVerifyMu.Unlock()
	binding := m.autoVerifyBinding()
	var verified, cooldown string
	storeGetJSON(m.store, "e2ee_verified_binding", &verified)
	if binding != "" && verified == binding {
		return true
	}
	if m.userID == "" || m.deviceID == "" || m.olmAccount == nil {
		return false
	}
	storeGetJSON(m.store, "e2ee_autoverify_cooldown", &cooldown)
	if cooldown == binding {
		return false
	}
	verifier := matrixSASVerifier{target: m, deadline: time.Now().Add(120 * time.Second)}
	result := verifier.run()
	if !result {
		_ = m.store.Set("e2ee_autoverify_cooldown", binding, 15*time.Minute)
	}
	return result
}
func (m *MatrixTarget) refreshVerifiedState() bool {
	if !m.autoVerifyMu.TryLock() {
		return false
	}
	defer m.autoVerifyMu.Unlock()
	binding := m.autoVerifyBinding()
	var verified string
	storeGetJSON(m.store, "e2ee_verified_binding", &verified)
	if binding == "" || verified != binding || m.olmAccount == nil || m.userID == "" || m.deviceID == "" {
		return false
	}
	if binding != m.e2eeBinding() {
		return false
	}
	var refreshed float64
	storeGetJSON(m.store, "e2ee_autoverify_refreshed_at", &refreshed)
	if !math.IsNaN(refreshed) && !math.IsInf(refreshed, 0) && float64(time.Now().Unix())-refreshed < 86400 {
		return true
	}
	identity, signing := m.olmAccount.PrivateKeys()
	for _, entry := range []struct {
		key   string
		value any
	}{{"device_id", m.deviceID}, {"e2ee_account", map[string][]byte{"identity": identity, "signing": signing}}, {"e2ee_device_binding", binding}, {"e2ee_verified_binding", binding}} {
		if err := m.store.Set(entry.key, entry.value, matrixE2EECacheTTL); err != nil {
			return false
		}
	}
	_ = m.store.Set("e2ee_autoverify_refreshed_at", float64(time.Now().Unix()), matrixE2EECacheTTL)
	return true
}

type matrixSASVerifier struct {
	target              *MatrixTarget
	deadline            time.Time
	transaction, device string
	peerKeys            map[string]string
	sas                 *matrixolm.SASVerification
	peerDone            bool
}

func (v *matrixSASVerifier) fetch(path string, payload map[string]any, params url.Values, method string, bestEffort bool) (map[string]any, bool) {
	remaining := time.Until(v.deadline)
	if remaining <= 0 && !bestEffort {
		return nil, false
	}
	if remaining < 0 {
		remaining = 0
	}
	currentHTTPOptionsMu.RLock()
	poll := currentHTTPOptions.readTimeout / 2
	currentHTTPOptionsMu.RUnlock()
	if poll < 250*time.Millisecond {
		poll = 250 * time.Millisecond
	}
	timeout := poll
	if remaining < timeout {
		timeout = remaining
	}
	timeout += 2 * time.Second
	endpoint, err := v.target.buildURL(path, params, "")
	if err != nil {
		return nil, false
	}
	body := ""
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, false
		}
		body = string(raw)
	}
	spec := RequestSpec{Method: method, URL: endpoint, Body: body, Headers: map[string]string{"User-Agent": "Apprise", "Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + v.target.accessToken}}
	request, err := spec.HTTPRequest()
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	response, err := httpClient().Do(request.WithContext(ctx))
	if err != nil {
		return nil, false
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, false
	}
	var data map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if !json.Valid(raw) || decoder.Decode(&data) != nil {
		return nil, false
	}
	return data, data != nil
}
func (v *matrixSASVerifier) run() bool {
	since := ""
	currentHTTPOptionsMu.RLock()
	poll := currentHTTPOptions.readTimeout / 2
	currentHTTPOptionsMu.RUnlock()
	if poll < 250*time.Millisecond {
		poll = 250 * time.Millisecond
	}
	for time.Now().Before(v.deadline) {
		if deadline := currentNotificationDeadline(); !deadline.IsZero() && !time.Now().Before(deadline) {
			return v.fail("m.timeout", "SAS verification timed out")
		}
		wait := poll
		if remaining := time.Until(v.deadline); remaining < wait {
			wait = remaining
		}
		params := url.Values{"timeout": {strconv.FormatInt(wait.Milliseconds(), 10)}, "filter": {matrixSASFilter}, "set_presence": {"offline"}}
		if since != "" {
			params.Set("since", since)
		}
		response, ok := v.fetch("/sync", nil, params, http.MethodGet, false)
		if !ok {
			return false
		}
		since = matrixSASString(response, "next_batch")
		if since == "" {
			return false
		}
		events, _ := matrixSASMap(response["to_device"])["events"].([]any)
		for _, raw := range events {
			if result, terminal := v.handle(matrixSASMap(raw)); terminal {
				return result
			}
			if !time.Now().Before(v.deadline) {
				break
			}
		}
	}
	return v.fail("m.timeout", "SAS verification timed out")
}
func (v *matrixSASVerifier) send(event string, content map[string]any, bestEffort bool) bool {
	return v.sendTo(event, v.device, content, bestEffort)
}
func (v *matrixSASVerifier) sendTo(event, device string, content map[string]any, bestEffort bool) bool {
	_, ok := v.fetch("/sendToDevice/"+matrixSASEvent+event+"/"+strings.ReplaceAll(newUUIDv4(), "-", ""), map[string]any{"messages": map[string]any{v.target.userID: map[string]any{device: content}}}, nil, http.MethodPut, bestEffort)
	return ok
}
func (v *matrixSASVerifier) fail(code, reason string) bool {
	if v.transaction != "" {
		v.send("cancel", map[string]any{"transaction_id": v.transaction, "code": code, "reason": reason}, true)
	}
	return false
}
func (v *matrixSASVerifier) storeVerified() bool {
	binding := v.target.autoVerifyBinding()
	return binding != "" && v.target.store.Set("e2ee_verified_binding", binding, matrixE2EECacheTTL) == nil
}
func (v *matrixSASVerifier) handle(event map[string]any) (bool, bool) {
	if matrixSASString(event, "sender") != v.target.userID {
		return false, false
	}
	content := matrixSASMap(event["content"])
	if content == nil {
		return false, false
	}
	kind := strings.TrimPrefix(matrixSASString(event, "type"), matrixSASEvent)
	transaction := matrixSASString(content, "transaction_id")
	if kind == "request" {
		device := matrixSASString(content, "from_device")
		number, ok := content["timestamp"].(json.Number)
		timestamp, parseError := strconv.ParseInt(string(number), 10, 64)
		now := time.Now().UnixMilli()
		if !ok || parseError != nil || timestamp < now-600000 || timestamp > now+300000 || !matrixSASID(transaction) || !matrixSASID(device) || !matrixSASHas(content["methods"], matrixolm.SASMethod) {
			return false, false
		}
		if v.transaction != "" {
			if device == v.device && transaction == v.transaction {
				return false, false
			}
			if device == v.device {
				v.fail("m.unexpected_message", "A newer verification request superseded this one")
				v.transaction = ""
				v.sas = nil
			}
			v.sendTo("cancel", device, map[string]any{"transaction_id": transaction, "code": "m.unexpected_message", "reason": "Another verification is already active"}, false)
			return false, false
		}
		keys := v.queryPeerKeys(device)
		if len(keys) == 0 {
			return false, true
		}
		v.transaction = transaction
		v.device = device
		v.peerKeys = keys
		if !v.send("ready", map[string]any{"transaction_id": transaction, "from_device": v.target.deviceID, "methods": []string{matrixolm.SASMethod}}, false) {
			return false, true
		}
		return false, false
	}
	if v.transaction == "" || transaction != v.transaction {
		return false, false
	}
	if kind == "cancel" {
		return false, true
	}
	if kind == "start" {
		if v.sas != nil {
			return v.fail("m.unexpected_message", "SAS verification was already started"), true
		}
		sas, err := matrixolm.NewSASVerification(v.target.userID, v.target.deviceID, v.target.userID, v.device, content)
		if err != nil {
			return v.fail("m.unknown_method", "Unsupported SAS verification parameters"), true
		}
		accept, err := sas.AcceptContent()
		if err != nil {
			return false, true
		}
		v.sas = sas
		if !v.send("accept", accept, false) {
			return false, true
		}
		return false, false
	}
	if v.sas == nil {
		return v.fail("m.unexpected_message", "SAS verification has not started"), true
	}
	switch kind {
	case "key":
		if err := v.sas.ReceiveKey(matrixSASString(content, "key")); err != nil {
			return v.fail("m.invalid_message", "Invalid SAS public key"), true
		}
		code, err := v.sas.DecimalSAS()
		if err != nil {
			return false, true
		}
		slog.Info("Matrix E2EE SAS code (compare against your other device)", "code", strconv.Itoa(code[0])+"-"+strconv.Itoa(code[1])+"-"+strconv.Itoa(code[2]))
		key, err := v.sas.KeyContent()
		if err != nil {
			return false, true
		}
		mac, err := v.sas.MACContent(v.target.olmAccount.SigningKey())
		if err != nil {
			return false, true
		}
		if !v.send("key", key, false) || !v.send("mac", mac, false) {
			return false, true
		}
	case "mac":
		if err := v.sas.VerifyPeerMAC(content, v.peerKeys); err != nil {
			return v.fail("m.key_mismatch", "SAS device key MAC did not match"), true
		}
		if !v.send("done", map[string]any{"transaction_id": v.transaction}, false) {
			return false, true
		}
		if v.peerDone {
			return v.storeVerified(), true
		}
	case "done":
		v.peerDone = true
		if v.sas.Verified() {
			return v.storeVerified(), true
		}
	}
	return false, false
}
func (v *matrixSASVerifier) queryPeerKeys(device string) map[string]string {
	response, ok := v.fetch("/keys/query", map[string]any{"device_keys": map[string]any{v.target.userID: []string{device}}}, nil, http.MethodPost, false)
	if !ok {
		return nil
	}
	info := matrixSASMap(matrixSASMap(matrixSASMap(response["device_keys"])[v.target.userID])[device])
	if !matrixVerifyDeviceKeys(info, v.target.userID, device) {
		return nil
	}
	result := map[string]string{}
	add := func(raw any) {
		keys := matrixSASMap(raw)
		if len(keys) <= 8 {
			for id, key := range keys {
				if value, ok := key.(string); ok {
					result[id] = value
				}
			}
		}
	}
	add(info["keys"])
	add(matrixSASMap(matrixSASMap(response["master_keys"])[v.target.userID])["keys"])
	return result
}
