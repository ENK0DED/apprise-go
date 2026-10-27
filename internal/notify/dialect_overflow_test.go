package notify

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestDialectOverflowUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/dialect_overflow.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Input, Dialect, Truncate string
		Limit                    int
		Split                    []string
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for i, v := range rows {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			convert := commonMarkToSlack
			format := "markdown"
			if v.Dialect == "chat" {
				convert = commonMarkToGoogleChat
			}
			if v.Dialect == "telegram-html" {
				convert = reduceTelegramHTML
				format = "html"
			}
			if v.Dialect == "slack-text" {
				convert = escapeSlackText
				format = "text"
			}
			got := fitDialect(v.Input, v.Limit, format, false, convert)
			if !reflect.DeepEqual(got, v.Split) {
				t.Fatalf("%s limit %d input %q\nGo %#v\nPython %#v", v.Dialect, v.Limit, v.Input, got, v.Split)
			}
			got = fitDialect(v.Input, v.Limit, format, true, convert)
			if len(got) != 1 || got[0] != v.Truncate {
				t.Fatalf("truncate Go %#v Python %q", got, v.Truncate)
			}
		})
	}
}
