package notify

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestCommonMarkRepairUpstreamVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/commonmark_repair.json.gz")
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
	var chains [][]struct {
		Input, Next, Output string
		State               map[string]any
	}
	if err = json.Unmarshal(data, &chains); err != nil {
		t.Fatal(err)
	}
	for i, chain := range chains {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			state := cmRepairState{}
			for j, v := range chain {
				got, next := repairCommonMark(v.Input, state, v.Next, -1)
				want := cmRepairState{}
				for k, x := range v.State {
					switch x := x.(type) {
					case bool:
						if x {
							want[k] = 1
						}
					case float64:
						want[k] = int(x)
					}
				}
				if got != v.Output || !reflect.DeepEqual(next, want) {
					t.Fatalf("piece %d input %q next %q prior %+v\nGo %q %+v\nPython %q %+v", j, v.Input, v.Next, state, got, next, v.Output, want)
				}
				state = next
			}
		})
	}
}
