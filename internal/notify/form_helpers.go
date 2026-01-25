package notify

import (
	"net/url"
	"strings"
)

type formPair struct {
	key   string
	value string
}

func encodeFormPairs(pairs []formPair) string {
	var b strings.Builder
	for i, pair := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(url.QueryEscape(pair.key))
		b.WriteByte('=')
		b.WriteString(url.QueryEscape(pair.value))
	}
	return b.String()
}
