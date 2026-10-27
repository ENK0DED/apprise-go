package notify

import (
	"regexp"
	"sort"
	"strings"
)

var cmWhatsAppTicks = regexp.MustCompile("`{3,}")

func cmWhatsAppURL(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		if r[i] == '\\' && i+1 < len(r) && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r[i+1]) {
			i++
		}
		b.WriteRune(r[i])
	}
	return strings.ReplaceAll(b.String(), ")", "%29")
}
func commonMarkToWhatsAppScanner(content string) string {
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
			if end > j {
				write("```" + cmWhatsAppTicks.ReplaceAllString(string(body[j:end-(j-i)]), "``") + "```")
			} else {
				write(string(body[i:end]))
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
				destination := cmWhatsAppURL(string(body[i+3 : close]))
				open := links[len(links)-1]
				links = links[:len(links)-1]
				if open == len(tokens)-1 {
					tokens[open].text = destination
				} else {
					tokens[open].text = ""
					write(" (" + destination + ")")
				}
				i = close + 2
				continue
			}
		}
		if r == ']' && i+1 < n && body[i+1] == '(' && len(links) > 0 {
			if close := cmDialectScanParen(body, i+1, &budget); close >= 0 {
				open := links[len(links)-1]
				links = links[:len(links)-1]
				destination := cmWhatsAppURL(string(body[i+2 : close]))
				if open == len(tokens)-1 {
					tokens[open].text = destination
				} else {
					tokens[open].text = ""
					write(" (" + destination + ")")
				}
				i = close + 1
				continue
			}
			links = links[:len(links)-1]
		} else if r == ']' && len(links) > 0 {
			links = links[:len(links)-1]
		}
		if r == '<' {
			close, valid := cmAuto(body, i)
			if close >= 0 {
				write(string(body[i+1 : close]))
				i = close + 1
				continue
			}
			if valid {
				write(string(body[i:]))
				i = n
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
