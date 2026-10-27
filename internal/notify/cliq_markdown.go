package notify

import (
	"sort"
	"strings"
	"unicode"
)

// Cliq consumes CommonMark emphasis but uses its own single-marker dialect.
// The scanner mirrors upstream's bounded link work and linked delimiter list.
// Separate output tokens avoid sentinel collisions with arbitrary user text.
type cmDialectEvent struct{ open, strong bool }
type cmDialectDelimiter struct {
	char                rune
	remaining, original int
	open, close         bool
	extendable          bool
	events              []cmDialectEvent
}
type cmDialectToken struct {
	text      string
	delimiter int
}

func cmDialectWhitespace(r rune) bool {
	return r < 0 || strings.ContainsRune(" \t\n\r\f\v", r) || unicode.Is(unicode.Zs, r)
}
func cmDialectPunctuation(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }
func cmDialectFlanking(prev, next rune) (bool, bool) {
	return !cmDialectWhitespace(next) && (!cmDialectPunctuation(next) || cmDialectWhitespace(prev) || cmDialectPunctuation(prev)), !cmDialectWhitespace(prev) && (!cmDialectPunctuation(prev) || cmDialectWhitespace(next) || cmDialectPunctuation(next))
}

func cmDialectMatch(delimiters []cmDialectDelimiter) {
	n := len(delimiters)
	previous := make([]int, n)
	next := make([]int, n)
	for i := range delimiters {
		previous[i] = i - 1
		next[i] = i + 1
	}
	head := 0
	unlink := func(index int) {
		before, after := previous[index], next[index]
		if before < 0 {
			head = after
		} else {
			next[before] = after
		}
		if after < n {
			previous[after] = before
		}
	}
	type bucket struct {
		char      rune
		open      bool
		remainder int
	}
	bounds := map[bucket]int{}
	for closerIndex := head; closerIndex < n; {
		closer := &delimiters[closerIndex]
		if !closer.close {
			closerIndex = next[closerIndex]
			continue
		}
		key := bucket{closer.char, closer.open, closer.original % 3}
		bound, exists := bounds[key]
		if !exists {
			bound = -1
		}
		found := -1
		for index := previous[closerIndex]; index > bound; index = previous[index] {
			opener := &delimiters[index]
			ambiguous := (closer.open || opener.close) && closer.original%3 != 0 && (opener.original+closer.original)%3 == 0
			if opener.char == closer.char && opener.open && !ambiguous {
				found = index
				break
			}
		}
		if found < 0 {
			bounds[key] = closerIndex - 1
			if !closer.open {
				position := next[closerIndex]
				unlink(closerIndex)
				closerIndex = position
			} else {
				closerIndex = next[closerIndex]
			}
			continue
		}
		opener := &delimiters[found]
		width := 1
		if closer.remaining >= 2 && opener.remaining >= 2 {
			width = 2
		}
		opener.remaining -= width
		closer.remaining -= width
		opener.events = append(opener.events, cmDialectEvent{true, width == 2})
		closer.events = append(closer.events, cmDialectEvent{false, width == 2})
		if found+1 != closerIndex {
			next[found] = closerIndex
			previous[closerIndex] = found
		}
		if opener.remaining == 0 {
			unlink(found)
		}
		if closer.remaining == 0 {
			position := next[closerIndex]
			unlink(closerIndex)
			closerIndex = position
		}
	}
	for pos := head; pos < n; pos = next[pos] {
		delimiters[pos].extendable = true
	}
}

func cmDialectScanAngle(body []rune, index int, budget *int) int {
	start := index + 3
	k := start
	limit := len(body)
	if *budget <= 0 {
		limit = minInt(limit, start+258)
	}
	for k < limit-1 {
		if body[k] == '\\' && k+1 < len(body) {
			k += 2
			continue
		}
		if body[k] == '>' && body[k+1] == ')' {
			*budget -= k - start
			return k
		}
		k++
	}
	*budget -= k - start
	return -1
}
func cmDialectScanParen(body []rune, index int, budget *int) int {
	start := index + 1
	k := start
	depth := 1
	limit := len(body)
	if *budget <= 0 {
		limit = minInt(limit, start+257)
	}
	for k < limit {
		r := body[k]
		if r == '\\' && k+1 < len(body) {
			k += 2
			continue
		}
		if r == '(' {
			depth++
		} else if r == ')' {
			depth--
			if depth == 0 {
				*budget -= k - start
				return k
			}
		} else if r == '<' || r < 0x20 || r == 0x7f || cmDialectWhitespace(r) {
			*budget -= k - start
			return -1
		}
		k++
	}
	*budget -= k - start
	return -1
}
func cmDialectAutolink(body []rune, index int) int {
	start := index + 1
	if start >= len(body) || !cmASCIIAlpha(body[start]) {
		return -1
	}
	k := start + 1
	for k < len(body) && k-start < 32 && ((body[k] >= 'a' && body[k] <= 'z') || (body[k] >= 'A' && body[k] <= 'Z') || (body[k] >= '0' && body[k] <= '9') || strings.ContainsRune("+.-", body[k])) {
		k++
	}
	if k >= len(body) || k-start < 2 || body[k] != ':' {
		return -1
	}
	for k++; k < len(body); k++ {
		if body[k] == '>' {
			return k
		}
		if body[k] == '<' || body[k] < 0x20 || body[k] == 0x7f || cmDialectWhitespace(body[k]) {
			return -1
		}
	}
	return -1
}

func commonMarkToCliq(content string) string {
	body := []rune(content)
	n := len(body)
	backticks := map[int][]int{}
	for i := 0; i < n; {
		if body[i] == '\\' && i+1 < n {
			i += 2
			continue
		}
		if body[i] == '`' {
			j := i + 1
			for j < n && body[j] == '`' {
				j++
			}
			backticks[j-i] = append(backticks[j-i], i)
			i = j
		} else {
			i++
		}
	}
	tokens := make([]cmDialectToken, 0, n)
	delimiters := []cmDialectDelimiter{}
	links := 0
	budget := n * 4
	write := func(text string) { tokens = append(tokens, cmDialectToken{text: text, delimiter: -1}) }
	for i := 0; i < n; {
		r := body[i]
		if r == '\\' && i+1 < n {
			write(string(body[i+1]))
			i += 2
			continue
		}
		if r == '`' {
			j := i + 1
			for j < n && body[j] == '`' {
				j++
			}
			positions := backticks[j-i]
			pos := sort.SearchInts(positions, j)
			end := j
			if pos < len(positions) {
				end = positions[pos] + j - i
			}
			write(string(body[i:end]))
			i = end
			continue
		}
		if r == '[' {
			links++
			write("[")
			i++
			continue
		}
		if r == ']' && i+2 < n && body[i+1] == '(' && body[i+2] == '<' && links > 0 {
			if close := cmDialectScanAngle(body, i, &budget); close >= 0 {
				destination := strings.ReplaceAll(string(body[i+3:close]), ")", "%29")
				links--
				write("](" + destination + ")")
				i = close + 2
				continue
			}
		}
		if r == ']' && i+1 < n && body[i+1] == '(' && links > 0 {
			if close := cmDialectScanParen(body, i+1, &budget); close >= 0 {
				links--
				write(string(body[i : close+1]))
				i = close + 1
				continue
			}
			links--
		} else if r == ']' && links > 0 {
			links--
		}
		if r == '<' {
			if close := cmDialectAutolink(body, i); close >= 0 {
				write(string(body[i+1 : close]))
				i = close + 1
				continue
			}
		}
		if r == '*' {
			j := i + 1
			for j < n && body[j] == r {
				j++
			}
			prev, next := rune(-1), rune(-1)
			if i > 0 {
				prev = body[i-1]
			}
			if j < n {
				next = body[j]
			}
			open, close := cmDialectFlanking(prev, next)
			tokens = append(tokens, cmDialectToken{delimiter: len(delimiters)})
			delimiters = append(delimiters, cmDialectDelimiter{char: r, remaining: j - i, original: j - i, open: open, close: close})
			i = j
			continue
		}
		write(string(r))
		i++
	}
	cmDialectMatch(delimiters)
	var out strings.Builder
	marker := func(e cmDialectEvent) string {
		if e.strong {
			return "*"
		}
		return "_"
	}
	for _, token := range tokens {
		if token.delimiter < 0 {
			out.WriteString(token.text)
			continue
		}
		d := delimiters[token.delimiter]
		for _, event := range d.events {
			if !event.open {
				out.WriteString(marker(event))
			}
		}
		out.WriteString(strings.Repeat(string(d.char), d.remaining))
		for i := len(d.events) - 1; i >= 0; i-- {
			if d.events[i].open {
				out.WriteString(marker(d.events[i]))
			}
		}
	}
	return out.String()
}

func cmASCIIAlpha(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
func cmASCIISchemeChar(r rune) bool {
	return cmASCIIAlpha(r) || r >= '0' && r <= '9' || strings.ContainsRune("+.-", r)
}
