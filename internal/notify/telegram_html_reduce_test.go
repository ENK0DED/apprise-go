package notify

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestTelegramHTMLReducerUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/telegram_html_reduce.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Input, Output string }
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := reduceTelegramHTML(row.Input); got != row.Output {
				t.Fatalf("input %q\nGo %q\nPython %q", row.Input, got, row.Output)
			}
		})
	}
}
