package parity

import (
	"encoding/json"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/unraid/apprise-go/internal/notify"
)

var headerDrop = map[string]struct{}{
	"x-apprise-id":              {},
	"x-apprise-recursion-count": {},
}

var headerKeep = map[string]struct{}{
	"user-agent":    {},
	"content-type":  {},
	"accept":        {},
	"accepts":       {},
	"authorization": {},
}

func assertRequestSpecMatches(t *testing.T, pythonSpec, goSpec notify.RequestSpec) {
	t.Helper()

	if strings.ToUpper(pythonSpec.Method) != strings.ToUpper(goSpec.Method) {
		t.Fatalf("method mismatch: python=%s go=%s", pythonSpec.Method, goSpec.Method)
	}

	pythonURL, err := url.Parse(pythonSpec.URL)
	if err != nil {
		t.Fatalf("parse python url: %v", err)
	}
	goURL, err := url.Parse(goSpec.URL)
	if err != nil {
		t.Fatalf("parse go url: %v", err)
	}
	if pythonURL.Scheme != goURL.Scheme || pythonURL.Host != goURL.Host || pythonURL.Path != goURL.Path || pythonURL.Fragment != goURL.Fragment {
		t.Fatalf("url mismatch: python=%s go=%s", pythonSpec.URL, goSpec.URL)
	}
	pythonQuery := pythonURL.Query().Encode()
	goQuery := goURL.Query().Encode()
	if pythonQuery != goQuery {
		t.Fatalf("url query mismatch: python=%s go=%s", pythonQuery, goQuery)
	}

	pythonHeaders := normalizeHeaders(pythonSpec.Headers)
	goHeaders := normalizeHeaders(goSpec.Headers)
	if !reflect.DeepEqual(pythonHeaders, goHeaders) {
		t.Fatalf("header mismatch: python=%v go=%v", pythonHeaders, goHeaders)
	}

	if shouldCompareJSON(pythonHeaders) && strings.TrimSpace(pythonSpec.Body) != "" && strings.TrimSpace(goSpec.Body) != "" {
		assertJSONBodyEqual(t, pythonSpec.Body, goSpec.Body)
		return
	}
	if shouldCompareForm(pythonHeaders, pythonSpec.Body) {
		assertQueryEqual(t, pythonSpec.Body, goSpec.Body)
		return
	}

	if pythonSpec.Body != goSpec.Body {
		t.Fatalf("body mismatch: python=%s go=%s", pythonSpec.Body, goSpec.Body)
	}
}

func assertRequestSpecSequenceMatches(t *testing.T, pythonSpecs, goSpecs []notify.RequestSpec) {
	t.Helper()

	if len(pythonSpecs) != len(goSpecs) {
		t.Fatalf("request count mismatch: python=%d go=%d", len(pythonSpecs), len(goSpecs))
	}

	for i := range pythonSpecs {
		assertRequestSpecMatches(t, pythonSpecs[i], goSpecs[i])
	}
}

func logProgress(t *testing.T, label string) {
	t.Helper()
	t.Logf("parity: %s", label)
}

func normalizeHeaders(headers map[string]string) map[string]string {
	normalized := map[string]string{}
	for key, value := range headers {
		lower := strings.ToLower(key)
		if _, drop := headerDrop[lower]; drop {
			continue
		}
		if _, keep := headerKeep[lower]; keep || strings.HasPrefix(lower, "x-") {
			normalized[lower] = value
		}
	}

	sorted := make([]string, 0, len(normalized))
	for key := range normalized {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)

	ordered := map[string]string{}
	for _, key := range sorted {
		ordered[key] = normalized[key]
	}

	return ordered
}

func shouldCompareJSON(headers map[string]string) bool {
	contentType := strings.ToLower(headers["content-type"])
	return strings.Contains(contentType, "application/json")
}

func shouldCompareForm(headers map[string]string, body string) bool {
	contentType := strings.ToLower(headers["content-type"])
	if !strings.Contains(contentType, "application/x-www-form-urlencoded") {
		return false
	}
	return strings.Contains(body, "=")
}

func assertJSONBodyEqual(t *testing.T, pythonBody, goBody string) {
	t.Helper()

	var pythonValue any
	var goValue any
	if err := json.Unmarshal([]byte(pythonBody), &pythonValue); err != nil {
		t.Fatalf("parse python json body: %v", err)
	}
	if err := json.Unmarshal([]byte(goBody), &goValue); err != nil {
		t.Fatalf("parse go json body: %v", err)
	}

	if !reflect.DeepEqual(pythonValue, goValue) {
		t.Fatalf("json body mismatch: python=%v go=%v", pythonValue, goValue)
	}
}

func assertQueryEqual(t *testing.T, pythonBody, goBody string) {
	t.Helper()

	pythonValues, err := url.ParseQuery(pythonBody)
	if err != nil {
		t.Fatalf("parse python query: %v", err)
	}
	goValues, err := url.ParseQuery(goBody)
	if err != nil {
		t.Fatalf("parse go query: %v", err)
	}

	if pythonValues.Encode() != goValues.Encode() {
		t.Fatalf("query mismatch: python=%s go=%s", pythonValues.Encode(), goValues.Encode())
	}
}
