package notify

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestSmartSplitUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/smart_split.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err = io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Input, Format string
		Limit         int
		Output        []string
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if got := smartSplit(row.Input, row.Limit, row.Format); !reflect.DeepEqual(got, row.Output) {
			t.Fatalf("%d %s limit%d input%q: Go%#v Python%#v", i, row.Format, row.Limit, row.Input, got, row.Output)
		}
	}
}
func TestEmojisBeforeFormatConversion(t *testing.T) {
	parsed := &ParsedURL{Scheme: "slack", Query: map[string]string{"emojis": "yes", "format": "markdown"}}
	got, err := ConvertMessageFormatForTarget(parsed, ":thumbs_up: *text*", "text")
	if err != nil {
		t.Fatal(err)
	}
	want, err := ConvertMessageFormat(ApplyEmojis(":thumbs_up: *text*"), "text", "markdown")
	if err != nil || got != want {
		t.Fatalf("got%q want%q error%v", got, want, err)
	}
}
