package notify

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	nethtml "golang.org/x/net/html"
)

var htmlWSCondense = regexp.MustCompile(`[\s]+`)

var htmlBlockTags = map[string]struct{}{
	"p":     {},
	"h1":    {},
	"h2":    {},
	"h3":    {},
	"h4":    {},
	"h5":    {},
	"h6":    {},
	"div":   {},
	"td":    {},
	"th":    {},
	"code":  {},
	"pre":   {},
	"label": {},
	"li":    {},
}

var htmlIgnoreTags = map[string]struct{}{
	"form":     {},
	"input":    {},
	"textarea": {},
	"select":   {},
	"ul":       {},
	"ol":       {},
	"style":    {},
	"link":     {},
	"meta":     {},
	"title":    {},
	"html":     {},
	"head":     {},
	"script":   {},
}

type htmlChunk struct {
	text     string
	blockEnd bool
}

type htmlTextConverter struct {
	doStore bool
	result  []htmlChunk
}

func markdownToHTML(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	// Match Python-Markdown notification defaults: tables and hard breaks.
	// Fenced blocks and typographic punctuation are not enabled upstream.
	extensions := parser.NoIntraEmphasis | parser.Tables | parser.HardLineBreak
	mdParser := parser.NewWithExtensions(extensions)
	mdParser.RegisterInline('`', notificationCodeSpan)
	renderer := html.NewRenderer(html.RendererOptions{
		Flags:          html.UseXHTML,
		RenderNodeHook: notificationHTMLBlockSpacing,
	})
	return strings.TrimSuffix(string(markdown.ToHTML([]byte(expandMarkdownTabs(content)), mdParser, renderer)), "\n")
}

func ConvertMessageFormat(content, inputFormat, outputFormat string) (string, error) {
	input := normalizeNotifyFormat(inputFormat)
	output := normalizeNotifyFormat(outputFormat)
	if input == "" {
		input = "text"
	}
	if output == "" {
		return content, nil
	}
	if !isSupportedNotifyFormat(input) {
		return "", fmt.Errorf("invalid input format: %s", inputFormat)
	}
	if !isSupportedNotifyFormat(output) {
		return "", fmt.Errorf("invalid output format: %s", outputFormat)
	}
	if input == output {
		return content, nil
	}

	switch input {
	case "markdown":
		if output == "html" {
			return markdownToHTML(content), nil
		}
		if output == "text" {
			return htmlToText(markdownToHTML(content)), nil
		}
	case "html":
		if output == "markdown" {
			return htmlToMarkdown(content), nil
		}
		if output == "text" {
			return htmlToText(content), nil
		}
	case "text":
		if output == "markdown" {
			return commonmarkTextEscapes.ReplaceAllStringFunc(content, func(match string) string { return "\\" + match }), nil
		}
		if output == "html" {
			return textToHTML(content), nil
		}
	}

	return content, nil
}

var commonmarkTextEscapes = regexp.MustCompile("[\\\\_*\\[\\]()~`>#+=|{}.!-]|&(?:#?[0-9A-Za-z]{1,32};)")

func isSupportedNotifyFormat(format string) bool {
	switch format {
	case "html", "markdown", "text":
		return true
	default:
		return false
	}
}

func textToHTML(content string) string {
	return escapeHTML(content, true, true)
}

func htmlToText(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	converter := &htmlTextConverter{doStore: true}
	tokenizer := nethtml.NewTokenizer(strings.NewReader(content))

	for {
		switch tokenizer.Next() {
		case nethtml.ErrorToken:
			return converter.finalize()
		case nethtml.TextToken:
			converter.handleData(string(tokenizer.Text()))
		case nethtml.StartTagToken, nethtml.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			converter.handleStartTag(strings.ToLower(string(name)))
		case nethtml.EndTagToken:
			name, _ := tokenizer.TagName()
			converter.handleEndTag(strings.ToLower(string(name)))
		}
	}
}

func escapeHTML(value string, convertNewLines bool, whitespace bool) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	escaped := replacer.Replace(value)

	if whitespace {
		escaped = strings.ReplaceAll(escaped, "\t", "&emsp;")
		escaped = strings.ReplaceAll(escaped, " ", "&nbsp;")
	}

	if convertNewLines {
		return strings.ReplaceAll(escaped, "\n", "<br/>")
	}
	return escaped
}

func (c *htmlTextConverter) handleData(data string) {
	if !c.doStore {
		return
	}
	content := htmlWSCondense.ReplaceAllString(data, " ")
	if content == "" {
		return
	}
	c.result = append(c.result, htmlChunk{text: content})
}

func (c *htmlTextConverter) handleStartTag(tag string) {
	_, ignore := htmlIgnoreTags[tag]
	c.doStore = !ignore

	if _, ok := htmlBlockTags[tag]; ok {
		c.result = append(c.result, htmlChunk{blockEnd: true})
	}

	switch tag {
	case "li":
		c.result = append(c.result, htmlChunk{text: "- "})
	case "br":
		c.result = append(c.result, htmlChunk{text: "\n"})
	case "hr":
		if len(c.result) > 0 {
			last := c.result[len(c.result)-1]
			if !last.blockEnd {
				last.text = strings.TrimRight(last.text, " ")
				c.result[len(c.result)-1] = last
			}
		}
		c.result = append(c.result, htmlChunk{text: "\n---\n"})
	case "blockquote":
		c.result = append(c.result, htmlChunk{text: " >"})
	}
}

func (c *htmlTextConverter) handleEndTag(tag string) {
	c.doStore = true
	if _, ok := htmlBlockTags[tag]; ok {
		c.result = append(c.result, htmlChunk{blockEnd: true})
	}
}

func (c *htmlTextConverter) finalize() string {
	var out strings.Builder
	var accum string
	hasAccum := false

	for _, item := range c.result {
		if item.blockEnd {
			if !hasAccum {
				continue
			}
			out.WriteString(strings.TrimSpace(accum))
			out.WriteString("\n")
			accum = ""
			hasAccum = false
			continue
		}

		if hasAccum {
			accum += item.text
		} else {
			accum = item.text
			hasAccum = true
		}
	}

	if hasAccum {
		out.WriteString(strings.TrimSpace(accum))
	}

	return strings.TrimSpace(out.String())
}

// Python-Markdown expands tabs to the next four-column stop before parsing.
func expandMarkdownTabs(content string) string {
	var out strings.Builder
	column := 0
	for _, r := range content {
		switch r {
		case '\t':
			n := 4 - column%4
			out.WriteString(strings.Repeat(" ", n))
			column += n
		case '\n', '\r':
			out.WriteRune(r)
			column = 0
		default:
			out.WriteRune(r)
			column++
		}
	}
	return out.String()
}

// The Markdown library treats three backticks as a fenced block even when
// that extension is disabled. Python-Markdown handles them as an inline span.
func notificationCodeSpan(_ *parser.Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]
	width := 0
	for width < len(data) && data[width] == '`' {
		width++
	}
	for i := width; i < len(data); {
		if data[i] != '`' {
			i++
			continue
		}
		end := i
		for end < len(data) && data[end] == '`' {
			end++
		}
		if end-i == width {
			return end, &ast.Code{Leaf: ast.Leaf{Literal: []byte(strings.Trim(string(data[width:i]), " \r\n"))}}
		}
		i = end
	}
	return 0, nil
}

// Python-Markdown separates generated root blocks with one source newline.
// Keep literal whitespace inside code and user-provided HTML untouched.
func notificationHTMLBlockSpacing(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	if entering {
		if _, root := node.GetParent().(*ast.Document); root {
			switch node.(type) {
			case *ast.Paragraph, *ast.Heading, *ast.List, *ast.BlockQuote, *ast.CodeBlock, *ast.HorizontalRule:
				if buffer, ok := w.(*bytes.Buffer); ok {
					buffer.Truncate(len(bytes.TrimRight(buffer.Bytes(), "\n")))
				}
			}
		}
	}
	return ast.GoToNext, false
}
