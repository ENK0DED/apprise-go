package notify

import (
	"regexp"
	"sort"
	"strings"
)

func cmAnchorEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
func cmAnchorURL(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); i++ {
		if r[i] == '\\' && i+1 < len(r) && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", r[i+1]) {
			i++
		}
		b.WriteRune(r[i])
	}
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "%7C").Replace(b.String())
}
func cmChatIndent(s string) string {
	lines := strings.Split(s, "\n")
	fence := 0
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		run := len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
		boundary := false
		if fence == 0 {
			if run >= 3 {
				fence = run
				boundary = true
			}
		} else if run >= fence && strings.TrimSpace(trimmed[run:]) == "" {
			fence = 0
			boundary = true
		}
		if !boundary && fence == 0 {
			lines[i] = strings.Repeat(" ", 2*(len(line)-len(trimmed))) + trimmed
		}
	}
	return strings.Join(lines, "\n")
}

var cmHeading = regexp.MustCompile(`^(#{1,6})(?:[ \t]+(.*)|$)`)
var cmHeadingClose = regexp.MustCompile(`[ \t]+#+[ \t]*$`)

// Preserve code fences and HTML blocks while translating ATX headings.
func commonMarkHeadingsToBold(s string) string {
	lines := strings.Split(s, "\n")
	fence := 0
	var fenceChar byte
	htmlBlock := false
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if htmlBlock {
			if strings.TrimSpace(line) == "" {
				htmlBlock = false
			}
			continue
		}
		if indent <= 3 && strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, "<http") {
			htmlBlock = true
			continue
		}
		run := 0
		var ch byte
		if len(trimmed) > 0 {
			ch = trimmed[0]
			if ch == '`' || ch == '~' {
				run = len(trimmed) - len(strings.TrimLeft(trimmed, string(ch)))
			}
		}
		if fence > 0 {
			if ch == fenceChar && run >= fence && strings.TrimSpace(trimmed[run:]) == "" {
				fence = 0
			}
			continue
		}
		if indent <= 3 && run >= 3 {
			fence = run
			fenceChar = ch
			continue
		}
		// Quotes and list markers remain outside the bold heading.
		prefix := line[:indent]
		rest := trimmed
		for strings.HasPrefix(rest, "> ") {
			prefix += "> "
			rest = rest[2:]
		}
		if len(rest) >= 2 && strings.ContainsRune("-*+", rune(rest[0])) && rest[1] == ' ' {
			prefix += rest[:2]
			rest = rest[2:]
		}
		if indent > 3 {
			continue
		}
		m := cmHeading.FindStringSubmatch(strings.TrimSuffix(rest, "\r"))
		if m == nil {
			continue
		}
		heading := strings.TrimSpace(cmHeadingClose.ReplaceAllString(m[2], ""))
		if heading == "" {
			lines[i] = prefix
		} else {
			lines[i] = prefix + "**" + heading + "**"
		}
	}
	return strings.Join(lines, "\n")
}
func commonMarkToAnchors(content string, chat bool) string {
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
			next := body[i+1]
			switch {
			case next == '<':
				write("&lt;")
			case next == '>':
				write("&gt;")
			case chat && next == '&':
				write("&amp;")
			case chat && strings.ContainsRune("*_~`", next):
				write("\\" + string(next))
			case chat || strings.ContainsRune("()[]!#", next):
				write(string(next))
			default:
				write(string(body[i : i+2]))
			}
			i += 2
			continue
		}
		if r == '&' {
			write("&amp;")
			i++
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
				write(string(body[i:j]) + cmAnchorEscape(string(body[j:end-(j-i)])) + string(body[i:j]))
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
				destination := cmAnchorURL(string(body[i+3 : close]))
				open := links[len(links)-1]
				links = links[:len(links)-1]
				tokens[open].text = "<" + destination + "|"
				write(">")
				i = close + 2
				continue
			}
		}
		if r == ']' && i+1 < n && body[i+1] == '(' && len(links) > 0 {
			if close := cmDialectScanParen(body, i+1, &budget); close >= 0 {
				open := links[len(links)-1]
				links = links[:len(links)-1]
				tokens[open].text = "<" + cmAnchorURL(string(body[i+2:close])) + "|"
				write(">")
				i = close + 1
				continue
			}
			links = links[:len(links)-1]
		} else if r == ']' && len(links) > 0 {
			links = links[:len(links)-1]
		}
		if r == '<' {
			if close := cmDialectAutolink(body, i); close >= 0 {
				write("<" + cmAnchorURL(string(body[i+1:close])) + ">")
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
	result := out.String()
	if chat {
		result = cmChatIndent(result)
	}
	return result
}
