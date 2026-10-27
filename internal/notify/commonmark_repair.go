package notify

import (
	"sort"
	"strings"
)

// CommonMark repair carries only verified open constructs between pieces.
// Tokens keep user characters separate from internal delimiter references.
type cmRepairState map[string]int

func cmLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("\\`*_[]()<>", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
func cmFlanking(char, prev, next rune) (bool, bool) {
	left, right := cmDialectFlanking(prev, next)
	if char == '_' {
		return left && (!right || cmDialectPunctuation(prev)), right && (!left || cmDialectPunctuation(next))
	}
	return left, right
}
func cmBackticks(body []rune) map[int][]int {
	index := map[int][]int{}
	for i := 0; i < len(body); {
		if body[i] == '\\' && i+1 < len(body) {
			i += 2
			continue
		}
		if body[i] == '`' {
			j := i + 1
			for j < len(body) && body[j] == '`' {
				j++
			}
			index[j-i] = append(index[j-i], i)
			i = j
		} else {
			i++
		}
	}
	return index
}
func cmBacktickClose(index map[int][]int, start, width int) int {
	positions := index[width]
	i := sort.SearchInts(positions, start)
	if i < len(positions) {
		return positions[i]
	}
	return -1
}
func cmAuto(body []rune, i int) (int, bool) {
	if close := cmDialectAutolink(body, i); close >= 0 {
		return close, true
	}
	suffix := string(body[i+1:])
	colon := strings.IndexByte(suffix, ':')
	if colon < 2 || colon > 32 {
		return -1, false
	}
	for j, r := range []rune(suffix) {
		if j == 0 && !cmASCIIAlpha(r) {
			return -1, false
		}
		if j < colon && !cmASCIISchemeChar(r) {
			return -1, false
		}
		if r == '<' || r < 0x20 || r == 0x7f || cmDialectWhitespace(r) {
			return -1, false
		}
	}
	return -1, true
}
func cmAngleEnd(body []rune, start int) (int, bool) {
	for i := start; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) {
			i++
			continue
		}
		if body[i] == '>' {
			if i+1 < len(body) && body[i+1] == ')' {
				return i, false
			}
			if i+1 == len(body) {
				return -1, true
			}
		}
	}
	return -1, false
}
func cmCloserWidths(body []rune, prev, boundary rune) map[rune]int {
	widths := map[rune]int{}
	ticks := cmBackticks(body)
	budget := len(body) * 4
	for i := 0; i < len(body); {
		r := body[i]
		if r == '\\' && i+1 < len(body) {
			i += 2
			continue
		}
		if r == '`' {
			j := i + 1
			for j < len(body) && body[j] == '`' {
				j++
			}
			close := cmBacktickClose(ticks, j, j-i)
			if close < 0 {
				break
			}
			i = close + j - i
			continue
		}
		if r == ']' && i+2 < len(body) && body[i+1] == '(' {
			var close int
			if i+2 < len(body) && body[i+2] == '<' {
				close = cmDialectScanAngle(body, i, &budget)
				if close >= 0 {
					i = close + 2
					continue
				}
			} else {
				close = cmDialectScanParen(body, i+1, &budget)
				if close >= 0 {
					i = close + 1
					continue
				}
			}
			break
		}
		if r == '<' {
			close, valid := cmAuto(body, i)
			if close >= 0 {
				i = close + 1
				continue
			}
			if valid {
				break
			}
		}
		if r == '*' || r == '_' {
			j := i + 1
			for j < len(body) && body[j] == r {
				j++
			}
			p, n := prev, boundary
			if i > 0 {
				p = body[i-1]
			}
			if j < len(body) {
				n = body[j]
			}
			_, canClose := cmFlanking(r, p, n)
			if canClose {
				widths[r] = max(widths[r], min(j-i, 3))
			}
			i = j
			continue
		}
		i++
	}
	return widths
}
func repairCommonMark(text string, state cmRepairState, next string, boundary rune) (string, cmRepairState) {
	body := []rune(text)
	lookahead := []rune(next)
	pending := cmRepairState{}
	for k, v := range state {
		pending[k] = v
	}
	pop := func(key string) int { v := pending[key]; delete(pending, key); return v }
	var tokens []cmDialectToken
	var delimiters []cmDialectDelimiter
	var links []int
	write := func(s string) { tokens = append(tokens, cmDialectToken{text: s, delimiter: -1}) }
	n := len(body)
	ticks := cmBackticks(body)
	i := 0
	budget := n * 4
	if width := pop("in_code"); width > 0 {
		close := cmBacktickClose(ticks, 0, width)
		if close < 0 {
			write(cmLiteral(text))
			pending["in_code"] = width
			i = n
		} else {
			write(cmLiteral(string(body[:close])))
			i = close + width
		}
	} else if pop("in_link_dest") > 0 {
		if pop("dest_gt") > 0 && n > 0 && body[0] == ')' {
			write("\\)")
			i = 1
		} else {
			close, trailing := cmAngleEnd(body, 0)
			if close >= 0 {
				write(cmLiteral(string(body[:close])) + "\\>\\)")
				i = close + 2
			} else {
				write(cmLiteral(text))
				pending["in_link_dest"] = 1
				if trailing {
					pending["dest_gt"] = 1
				}
				i = n
			}
		}
	} else if pop("in_autolink") > 0 {
		close := -1
		for j, r := range body {
			if r == '>' {
				close = j
				break
			}
		}
		if close >= 0 {
			write(cmLiteral(string(body[:close])) + "\\>")
			i = close + 1
		} else {
			write(cmLiteral(text))
			pending["in_autolink"] = 1
			i = n
		}
	} else if pop("in_escape") > 0 {
		if n > 0 {
			write(string(body[0]))
			i = 1
		} else {
			pending["in_escape"] = 1
		}
	}
	for i < n {
		r := body[i]
		if r == '\\' {
			if i+1 < n {
				write(string(body[i : i+2]))
				i += 2
			} else {
				write("\\")
				pending["in_escape"] = 1
				i = n
			}
			continue
		}
		if r == '`' {
			j := i + 1
			for j < n && body[j] == '`' {
				j++
			}
			close := cmBacktickClose(ticks, j, j-i)
			if close >= 0 {
				write(string(body[i : close+j-i]))
				i = close + j - i
			} else {
				pending["in_code"] = j - i
				write(cmLiteral(string(body[j:])))
				i = n
			}
			continue
		}
		if r == '<' {
			close, valid := cmAuto(body, i)
			if close >= 0 {
				write(string(body[i : close+1]))
				i = close + 1
				continue
			}
			if valid {
				write(cmLiteral(string(body[i:])))
				pending["in_autolink"] = 1
				i = n
				continue
			}
		}
		if r == '*' || r == '_' {
			j := i + 1
			for j < n && body[j] == r {
				j++
			}
			prev, nxt := rune(-1), rune(-1)
			if i > 0 {
				prev = body[i-1]
			}
			if j < n {
				nxt = body[j]
			} else if len(lookahead) > 0 {
				nxt = lookahead[0]
			}
			run := j - i
			for run > 0 {
				width := 1
				if run >= 2 {
					width = 2
				}
				marker := strings.Repeat(string(r), width)
				if pending[marker] <= 0 {
					break
				}
				pending[marker]--
				run -= width
			}
			if run > 0 {
				open, close := cmFlanking(r, prev, nxt)
				tokens = append(tokens, cmDialectToken{delimiter: len(delimiters)})
				delimiters = append(delimiters, cmDialectDelimiter{char: r, remaining: run, original: run, open: open, close: close})
			}
			i = j
			continue
		}
		if r == '[' {
			links = append(links, len(tokens))
			write("[")
			i++
			continue
		}
		if r == ']' && i+1 < n && body[i+1] == '(' {
			if i+2 < len(body) && body[i+2] == '<' {
				close := cmDialectScanAngle(body, i, &budget)
				if close >= 0 && len(links) > 0 {
					links = links[:len(links)-1]
					write(string(body[i : close+2]))
					i = close + 2
					continue
				}
				if len(links) > 0 {
					links = links[:len(links)-1]
				}
				if close >= 0 {
					write("\\]\\(" + cmLiteral(string(body[i+3:close])) + "\\>\\)")
					i = close + 2
				} else {
					write("\\]\\(" + cmLiteral(string(body[i+3:])))
					pending["in_link_dest"] = 1
					_, trailing := cmAngleEnd(body, i+3)
					if trailing {
						pending["dest_gt"] = 1
					}
					i = n
				}
				continue
			}
			close := cmDialectScanParen(body, i+1, &budget)
			if close >= 0 && len(links) > 0 {
				links = links[:len(links)-1]
				write(string(body[i : close+1]))
				i = close + 1
				continue
			}
			if len(links) > 0 {
				links = links[:len(links)-1]
			}
			if close >= 0 {
				write("\\]\\(" + cmLiteral(string(body[i+2:close])) + "\\)")
				i = close + 1
			} else {
				write(string(body[i:]))
				i = n
			}
			continue
		}
		write(string(r))
		i++
	}
	for _, index := range links {
		tokens[index].text = "\\" + tokens[index].text
	}
	cmDialectMatch(delimiters)
	prev := rune(-1)
	if n > 0 {
		prev = body[n-1]
	}
	widths := cmCloserWidths(lookahead, prev, boundary)
	forced := map[int]string{}
	var tails []string
	for index, d := range delimiters {
		available := widths[d.char]
		if !d.open || !d.extendable || d.remaining <= 0 || available == 0 {
			continue
		}
		if len(tokens) > 0 && tokens[len(tokens)-1].delimiter == index && index == len(delimiters)-1 && n != d.original {
			delimiters[index].remaining %= 2
			continue
		}
		remain := d.remaining
		var opened []string
		if available >= 2 && remain >= 2 {
			opened = append(opened, strings.Repeat(string(d.char), 2))
			remain -= 2
		}
		if available != 2 && remain >= 1 {
			opened = append(opened, string(d.char))
			remain--
		}
		close, _ := cmRepairEvents(d)
		forced[index] = close + cmLiteral(strings.Repeat(string(d.char), remain)) + strings.Join(opened, "")
		for _, marker := range opened {
			pending[marker]++
		}
		tails = append(tails, strings.Join(opened, ""))
	}
	var out strings.Builder
	for _, token := range tokens {
		if token.delimiter < 0 {
			out.WriteString(token.text)
			continue
		}
		index := token.delimiter
		d := delimiters[index]
		if s, ok := forced[index]; ok {
			out.WriteString(s)
			continue
		}
		close, open := cmRepairEvents(d)
		out.WriteString(close)
		out.WriteString(cmLiteral(strings.Repeat(string(d.char), d.remaining)))
		out.WriteString(open)
	}
	for i := len(tails) - 1; i >= 0; i-- {
		out.WriteString(tails[i])
	}
	return out.String(), pending
}
func cmRepairEvents(d cmDialectDelimiter) (string, string) {
	var close, open strings.Builder
	marker := func(e cmDialectEvent) string {
		width := 1
		if e.strong {
			width = 2
		}
		return strings.Repeat(string(d.char), width)
	}
	for _, e := range d.events {
		if !e.open {
			close.WriteString(marker(e))
		}
	}
	for i := len(d.events) - 1; i >= 0; i-- {
		if d.events[i].open {
			open.WriteString(marker(d.events[i]))
		}
	}
	return close.String(), open.String()
}
