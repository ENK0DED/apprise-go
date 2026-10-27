package notify

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestCliqMarkdownUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/cliq_markdown.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Input, Output string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for index, vector := range vectors {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			if got := commonMarkToCliq(vector.Input); got != vector.Output {
				t.Fatalf("input %q\nGo %q\nPython %q", vector.Input, got, vector.Output)
			}
		})
	}
}
