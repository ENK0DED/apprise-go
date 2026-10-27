package main

import (
	"encoding/json"
	"testing"
)

func TestLiteralPreservesLargeIntegerBounds(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"255", "255"}, {"4294967295", "int64(4294967295)"}, {"-2147483649", "int64(-2147483649)"}, {"4.0", "4.0"}} {
		if got := literal(json.Number(tc.input), 0); got != tc.want {
			t.Errorf("%s got%s want%s", tc.input, got, tc.want)
		}
	}
}
