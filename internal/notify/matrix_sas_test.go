package notify

import (
	"encoding/json"
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
