package notify

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFormatConversionReferenceVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/format_conversion.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, Source, Target, Input, Expected string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := ConvertMessageFormat(tc.Input, tc.Source, tc.Target)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.Expected {
				t.Fatalf("got %q; want upstream %q", got, tc.Expected)
			}
		})
	}
}
