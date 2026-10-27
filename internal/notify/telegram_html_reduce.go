package notify

import (
	nethtml "golang.org/x/net/html"

	"regexp"
	"strconv"
	"strings"
)

// Telegram's declared HTML path balances supported tags and preserves text
// while reducing block, list, table, and code markup to its supported subset.
type telegramReducer struct {
	out                  []string
	tags                 [][2]string
	count                map[string]int
	lists                []int
	skip, cells, checked int
	hasText              bool
}

var telegramLooseEntity = regexp.MustCompile(`(?i)&(apos|quot|nbsp|emsp)(?:;|\b)`)
var telegramSourceNewlines = regexp.MustCompile(`(?:\r\n|\r|\n)+`)
var telegramInline = map[string]string{"b": "b", "strong": "b", "i": "i", "em": "i", "u": "u", "ins": "u", "s": "s", "strike": "s", "del": "s", "tg-spoiler": "tg-spoiler"}

func telegramHeading(tag string) bool {
	return len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' || tag == "title"
}
func telegramBlock(tag string) bool {
	return tag == "p" || tag == "div" || tag == "table" || tag == "hr"
}
func telegramTextEscape(s string) string {
	return cmAnchorEscape(strings.NewReplacer("\u00a0", " ", "\u2003", "   ").Replace(s))
}
func telegramAttrEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#x27;").Replace(s)
}
func (r *telegramReducer) open(source, tag, attrs string) {
	r.out = append(r.out, "<"+tag+attrs+">")
	r.tags = append(r.tags, [2]string{source, tag})
	r.count[source]++
}
func (r *telegramReducer) close(source string) {
	if r.count[source] == 0 {
		return
	}
	for len(r.tags) > 0 {
		tag := r.tags[len(r.tags)-1]
		r.tags = r.tags[:len(r.tags)-1]
		r.count[tag[0]]--
		r.out = append(r.out, "</"+tag[1]+">")
		if tag[0] == source {
			break
		}
	}
}
func (r *telegramReducer) br() {
	if len(r.out) > 0 {
		i := len(r.out) - 1
		if !strings.HasPrefix(r.out[i], "<") && r.count["pre"] == 0 && r.count["code"] == 0 {
			r.out[i] = strings.TrimRight(r.out[i], " \t\r\n\v\f")
		}
	}
	r.out = append(r.out, "\r\n")
}
func (r *telegramReducer) line() {
	for _, part := range r.out[r.checked:] {
		if part == "\r\n" {
			r.hasText = false
		} else if !strings.HasPrefix(part, "<") {
			r.hasText = true
		}
	}
	r.checked = len(r.out)
	if r.hasText {
		r.br()
	}
}
func (r *telegramReducer) start(tag string, attrs map[string]string) {
	if tag == "script" || tag == "style" {
		r.skip++
		return
	}
	if tag == "br" {
		r.br()
		return
	}
	if r.count["code"] > 0 || r.count["pre"] > 0 && tag != "code" {
		return
	}
	switch {
	case tag == "span" && strings.Contains(" "+attrs["class"]+" ", " tg-spoiler "):
		r.open(tag, "tg-spoiler", "")
	case tag == "a" && attrs["href"] != "" && r.count["a"] == 0:
		r.open(tag, "a", " href=\""+telegramAttrEscape(attrs["href"])+"\"")
	case tag == "tg-emoji" && attrs["emoji-id"] != "":
		r.open(tag, tag, " emoji-id=\""+telegramAttrEscape(attrs["emoji-id"])+"\"")
	case tag == "pre" || tag == "code":
		attribute := ""
		if tag == "code" && r.count["pre"] > 0 && strings.HasPrefix(attrs["class"], "language-") {
			attribute = " class=\"" + telegramAttrEscape(attrs["class"]) + "\""
		}
		if tag == "pre" {
			r.line()
		}
		r.open(tag, tag, attribute)
	case tag == "blockquote":
		r.line()
		if r.count[tag] == 0 {
			attribute := ""
			if _, ok := attrs["expandable"]; ok {
				attribute = " expandable"
			}
			r.open(tag, tag, attribute)
		}
	case telegramBlock(tag):
		r.line()
	case telegramInline[tag] != "":
		r.open(tag, telegramInline[tag], "")
	case telegramHeading(tag):
		r.line()
		r.open(tag, "b", "")
	case tag == "ul" || tag == "ol":
		r.line()
		number := 0
		if tag == "ol" {
			number = 1
		}
		r.lists = append(r.lists, number)
	case tag == "li":
		r.line()
		number := 0
		if len(r.lists) > 0 {
			number = r.lists[len(r.lists)-1]
		}
		indent := strings.Repeat("  ", max(0, min(len(r.lists), 4)-1))
		if number == 0 {
			r.out = append(r.out, indent+"- ")
		} else {
			r.out = append(r.out, indent+strconv.Itoa(number)+". ")
			r.lists[len(r.lists)-1]++
		}
	case tag == "tr":
		r.line()
		r.cells = 0
	case tag == "td" || tag == "th":
		if r.cells > 0 {
			r.out = append(r.out, " | ")
		}
		r.cells++
	case tag == "img" && attrs["alt"] != "":
		r.out = append(r.out, cmAnchorEscape(attrs["alt"]))
	}
}
func (r *telegramReducer) end(tag string) {
	if tag == "script" || tag == "style" {
		r.skip = max(0, r.skip-1)
		return
	}
	if tag == "br" {
		r.br()
		return
	}
	r.close(tag)
	if (tag == "ul" || tag == "ol") && len(r.lists) > 0 {
		r.lists = r.lists[:len(r.lists)-1]
	}
	if telegramHeading(tag) || tag == "ul" || tag == "ol" || tag == "li" || tag == "tr" {
		r.line()
	}
	if telegramBlock(tag) || tag == "pre" || tag == "blockquote" {
		r.line()
	}
}
func (r *telegramReducer) write(data string) {
	if len(r.out) == 0 || r.out[len(r.out)-1] == "\r\n" {
		data = strings.TrimLeft(data, " \t\r\n\v\f")
	}
	if data != "" {
		r.out = append(r.out, telegramTextEscape(data))
	}
}
func (r *telegramReducer) text(data string) {
	if r.skip > 0 {
		return
	}
	if r.count["pre"] > 0 || r.count["code"] > 0 {
		if data != "" {
			r.out = append(r.out, telegramTextEscape(data))
		}
		return
	}
	if strings.TrimSpace(data) == "" {
		r.write(telegramSourceNewlines.ReplaceAllString(data, " "))
		return
	}
	start := 0
	for _, span := range telegramSourceNewlines.FindAllStringIndex(data, -1) {
		r.write(data[start:span[0]])
		segment := strings.ReplaceAll(strings.ReplaceAll(data[span[0]:span[1]], "\r\n", "\n"), "\r", "\n")
		count := strings.Count(segment, "\n")
		if len(r.out) == 0 || r.out[len(r.out)-1] == "\r\n" {
			count--
		}
		for i := 0; i < count; i++ {
			r.out = append(r.out, "\r\n")
		}
		start = span[1]
	}
	r.write(data[start:])
}
func reduceTelegramHTML(content string) string {
	content = telegramLooseEntity.ReplaceAllStringFunc(content, func(entity string) string { return strings.ToLower(strings.TrimSuffix(entity, ";")) + ";" })
	r := telegramReducer{count: map[string]int{}}
	tokens := nethtml.NewTokenizer(strings.NewReader(content))
	for {
		kind := tokens.Next()
		if kind == nethtml.ErrorToken {
			break
		}
		token := tokens.Token()
		switch kind {
		case nethtml.StartTagToken, nethtml.SelfClosingTagToken:
			if kind == nethtml.SelfClosingTagToken && token.Data != "br" && token.Data != "hr" && token.Data != "img" {
				continue
			}
			attrs := map[string]string{}
			for _, attr := range token.Attr {
				attrs[attr.Key] = attr.Val
			}
			r.start(token.Data, attrs)
		case nethtml.EndTagToken:
			r.end(token.Data)
		case nethtml.TextToken:
			r.text(token.Data)
		}
	}
	for len(r.out) > 0 && (r.out[len(r.out)-1] == "\r\n" || strings.TrimSpace(r.out[len(r.out)-1]) == "") {
		r.out = r.out[:len(r.out)-1]
	}
	for len(r.tags) > 0 {
		tag := r.tags[len(r.tags)-1]
		r.tags = r.tags[:len(r.tags)-1]
		r.out = append(r.out, "</"+tag[1]+">")
	}
	return strings.Join(r.out, "")
}
