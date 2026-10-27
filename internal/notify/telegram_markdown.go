package notify

import (
	"sort"
	"strings"
)

func cmTelegramURL(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		if r[i] == '\\' && i+1 < len(r) {
			b.WriteRune(r[i])
			i++
			b.WriteRune(r[i])
			continue
		}
		if r[i] == '(' || r[i] == ')' {
			b.WriteByte('\\')
		}
		b.WriteRune(r[i])
	}
	return b.String()
}
func cmTelegramEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("~#+=|{}.!<>-_*[]()`", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
func commonMarkToTelegram(content string, strict bool) string {
	content = commonMarkHeadingsToBold(content)
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
	var links []int
	orphans := map[int]string{}
	orphan := func(index int, marker string) {
		if strict {
			tokens[index].text = ""
			for _, r := range marker {
				tokens[index].text += "\\" + string(r)
			}
		} else {
			tokens[index].text = ""
			orphans[index] = marker
		}
	}
	budget := n * 4
	write := func(text string) { tokens = append(tokens, cmDialectToken{text: text, delimiter: -1}) }
	for i := 0; i < n; {
		r := body[i]
		if r == '\\' && i+1 < n {
			if strict || strings.ContainsRune("`*_[]", body[i+1]) {
				write(string(body[i : i+2]))
			} else {
				write(string(body[i+1]))
			}
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
			if end > j {
				span := strings.NewReplacer("\\", "\\\\", "`", "\\`").Replace(string(body[j : end-(j-i)]))
				delim := "`"
				if j-i >= 3 || strings.Contains(span, "\n") {
					delim = "```"
				}
				write(delim + span + delim)
			} else {
				index := len(tokens)
				write("")
				orphan(index, strings.Repeat("`", j-i))
			}
			i = end
			continue
		}
		if r == '[' {
			links = append(links, len(tokens))
			write("[")
			i++
			continue
		}
		if r == ']' && i+2 < n && body[i+1] == '(' && body[i+2] == '<' && len(links) > 0 {
			if close := cmDialectScanAngle(body, i, &budget); close >= 0 {
				destination := strings.NewReplacer("\\", "\\\\", ")", "\\)").Replace(string(body[i+3 : close]))
				links = links[:len(links)-1]
				write("](" + destination + ")")
				i = close + 2
				continue
			}
		}
		if r == ']' && i+1 < n && body[i+1] == '(' && len(links) > 0 {
			if close := cmDialectScanParen(body, i+1, &budget); close >= 0 {
				links = links[:len(links)-1]
				write("](" + cmTelegramURL(string(body[i+2:close])) + ")")
				i = close + 1
				continue
			}
			orphan(links[len(links)-1], "[")
			links = links[:len(links)-1]
		} else if r == ']' && len(links) > 0 {
			orphan(links[len(links)-1], "[")
			links = links[:len(links)-1]
		}
		if r == '<' {
			if close := cmDialectAutolink(body, i); close >= 0 {
				url := string(body[i+1 : close])
				if strict {
					url = cmTelegramEscape(url)
				}
				write(url)
				i = close + 1
				continue
			}
		}
		if strict && strings.ContainsRune("]()~#+=|{}.!<>-", r) {
			write("\\" + string(r))
			i++
			continue
		}
		if r == '*' || r == '_' {
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
			open, close := cmFlanking(r, prev, next)
			tokens = append(tokens, cmDialectToken{delimiter: len(delimiters)})
			delimiters = append(delimiters, cmDialectDelimiter{char: r, remaining: j - i, original: j - i, open: open, close: close})
			i = j
			continue
		}
		write(string(r))
		i++
	}

	for _, index := range links {
		orphan(index, "[")
	}
	cmDialectMatch(delimiters)
	type rendered struct {
		close, open, leftover string
		after                 int
	}
	renderedRuns := make([]rendered, len(delimiters))
	depth := 0
	for index, d := range delimiters {
		var close, open strings.Builder
		for _, e := range d.events {
			if !e.open {
				if strict || depth == 1 {
					if e.strong {
						close.WriteByte('*')
					} else {
						close.WriteByte('_')
					}
				}
				depth--
			}
		}
		marker := string(d.char)
		if strict || depth == 0 {
			marker = "\\" + marker
		}
		leftover := strings.Repeat(marker, d.remaining)
		for i := len(d.events) - 1; i >= 0; i-- {
			e := d.events[i]
			if e.open {
				if strict || depth == 0 {
					if e.strong {
						open.WriteByte('*')
					} else {
						open.WriteByte('_')
					}
				}
				depth++
			}
		}
		renderedRuns[index] = rendered{close.String(), open.String(), leftover, depth}
	}
	var out strings.Builder
	spanDepth := 0
	for index, token := range tokens {
		if marker, ok := orphans[index]; ok {
			if spanDepth == 0 {
				for _, r := range marker {
					out.WriteByte('\\')
					out.WriteRune(r)
				}
			} else {
				out.WriteString(marker)
			}
			continue
		}
		if token.delimiter < 0 {
			out.WriteString(token.text)
			continue
		}
		r := renderedRuns[token.delimiter]
		out.WriteString(r.close)
		out.WriteString(r.leftover)
		out.WriteString(r.open)
		spanDepth = r.after
	}
	return out.String()
}
