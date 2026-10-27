package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMatrixSASIgnoresForeignStaleAndMalformedRequests(t *testing.T) {
	for _, mutation := range []string{"foreign-user", "stale", "future", "fractional-time", "oversized-device", "oversized-transaction", "wrong-method", "missing-content"} {
		t.Run(mutation, func(t *testing.T) {
			content := map[string]any{"transaction_id": "transaction", "from_device": "PEER", "timestamp": json.Number(strconv.FormatInt(time.Now().UnixMilli(), 10)), "methods": []any{"m.sas.v1"}}
			event := map[string]any{"sender": "@same:example.com", "type": "m.key.verification.request", "content": content}
			switch mutation {
			case "foreign-user":
				event["sender"] = "@other:example.com"
			case "stale":
				content["timestamp"] = json.Number(strconv.FormatInt(time.Now().Add(-11*time.Minute).UnixMilli(), 10))
			case "future":
				content["timestamp"] = json.Number(strconv.FormatInt(time.Now().Add(6*time.Minute).UnixMilli(), 10))
			case "fractional-time":
				content["timestamp"] = json.Number(strconv.FormatInt(time.Now().UnixMilli(), 10) + ".0")
			case "oversized-device":
				content["from_device"] = strings.Repeat("x", 513)
			case "oversized-transaction":
				content["transaction_id"] = strings.Repeat("x", 513)
			case "wrong-method":
				content["methods"] = []any{"unsupported"}
			case "missing-content":
				delete(event, "content")
			}
			verifier := matrixSASVerifier{target: &MatrixTarget{userID: "@same:example.com"}}
			result, terminal := verifier.handle(event)
			if result || terminal || verifier.transaction != "" {
				t.Fatalf("malformed event affected verification: result%t terminal%t transaction%q", result, terminal, verifier.transaction)
			}
		})
	}
}

func TestMatrixSASPollUsesVerificationTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, `{"next_batch":"next"}`)
	}))
	defer server.Close()
	parsed, err := ParseURL("matrix://user:pass@" + strings.TrimPrefix(server.URL, "http://") + "/!room?discovery=no&e2ee=no")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewMatrixTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	options := defaultHTTPOptions()
	options.readTimeout = 100 * time.Millisecond
	restore := withHTTPOptions(options)
	defer restore()
	verifier := matrixSASVerifier{target: target, deadline: time.Now().Add(time.Second)}
	response, ok := verifier.fetch("/sync", nil, nil, http.MethodGet, false)
	if !ok || response["next_batch"] != "next" {
		t.Fatal("ordinary read timeout cut off the bounded verification poll")
	}
}
