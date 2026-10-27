package notify

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func dialectFor(schema, format string) func(string) string {
	switch schema {
	case "slack":
		if format == "markdown" {
			return commonMarkToSlack
		}
		if format == "text" {
			return escapeSlackText
		}
	case "gchat":
		if format == "markdown" {
			return commonMarkToGoogleChat
		}
	case "cliq", "zcliq", "zohocliq":
		if format == "markdown" {
			return commonMarkToCliq
		}
	case "evolution", "evolutions":
		if format == "markdown" {
			return commonMarkToWhatsApp
		}
	case "kook":
		if format == "markdown" {
			return commonMarkHeadingsToBold
		}
	}
	return nil
}
func applyDialectOverflow(parsed *ParsedURL, mode, format, inputFormat string, parts []MessagePart) []MessagePart {
	if parsed == nil || inputFormat == "" {
		return parts
	}
	convert := dialectFor(parsed.Scheme, format)
	if parsed.Scheme == "tgram" && parsed.Query["template"] == "" {
		switch format {
		case "markdown":
			strict := telegramMarkdownMode(parsed.Query["mdv"]) == "MarkdownV2"
			convert = func(body string) string { return commonMarkToTelegram(body, strict) }
		case "html":
			convert = reduceTelegramHTML
		}
	}
	if convert == nil {
		return parts
	}
	parsed.dialectApplied = true
	limits, known := overflowLimitsFor(parsed.Scheme)
	if resolved, ok := dynamicOverflowLimits(parsed.Scheme, parsed, limits); ok {
		limits = resolved
		known = true
	}
	var result []MessagePart
	for _, part := range parts {
		bodies := []string{convert(part.Body)}
		if known && limits.bodyMax > 0 && (mode == OverflowSplit || mode == OverflowTruncate) {
			bodies = fitDialect(part.Body, limits.bodyMax, format, mode == OverflowTruncate, convert)
		}
		repeat := !limits.amalgamateTitle || (limits.bodyMax >= limits.countThreshold && limits.displayTitleOnce == nil)
		if limits.displayTitleOnce != nil {
			repeat = !*limits.displayTitleOnce
		}
		for i, body := range bodies {
			title := part.Title
			if i > 0 && !repeat {
				title = ""
			}
			result = append(result, MessagePart{Body: body, Title: title})
		}
	}
	return result
}
func fitDialect(body string, limit int, format string, truncate bool, convert func(string) string) []string {
	if format != "markdown" {
		return fitDialectCuts(body, limit, format, truncate, convert)
	}
	if converted := convert(body); utf8.RuneCountInString(converted) <= limit {
		return []string{converted}
	}
	runes := []rune(body)
	offset := 0
	pending := cmRepairState{}
	var result []string
	for offset < len(runes) {
		fit := func(width int) (string, cmRepairState) {
			text := string(runes[offset : offset+width])
			state := pending
			if format == "markdown" {
				end := min(offset+width+min(max(1, limit*8), 2048), len(runes))
				boundary := rune(-1)
				if end < len(runes) {
					boundary = runes[end]
				}
				text, state = repairCommonMark(text, pending, string(runes[offset+width:end]), boundary)
			}
			return convert(text), state
		}
		best, width, state := "", 0, pending
		probe := min(len(runes)-offset, max(1, limit))
		firstFail := probe
		for {
			candidate, next := fit(probe)
			if utf8.RuneCountInString(candidate) > limit {
				firstFail = probe
				break
			}
			best, width, state = candidate, probe, next
			if probe == len(runes)-offset {
				break
			}
			probe = min(len(runes)-offset, probe*2)
		}
		lo, hi := width+1, firstFail-1
		for lo <= hi {
			mid := (lo + hi) / 2
			candidate, next := fit(mid)
			if utf8.RuneCountInString(candidate) <= limit {
				best, width, state = candidate, mid, next
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		end := min(len(runes)-offset, firstFail+min(min(max(1, limit*8), 2048), 24))
		if format == "markdown" && strings.ContainsAny(string(runes[offset+width:offset+end]), "\\`*_[]()<>") {
			up := min(end, width+24)
			for i := width + 1; i <= up; i++ {
				candidate, next := fit(i)
				if utf8.RuneCountInString(candidate) <= limit {
					best, width, state = candidate, i, next
				}
			}
			for i, count := end, 0; i > up && count < 8; i, count = i-1, count+1 {
				candidate, next := fit(i)
				if utf8.RuneCountInString(candidate) <= limit {
					best, width, state = candidate, i, next
					break
				}
			}
		}
		if width == 0 {
			best, state = fit(1)
			width = 1
		}
		if best != "" || len(result) == 0 {
			result = append(result, best)
		}
		offset += width
		pending = state
		if truncate {
			break
		}
	}
	if len(result) == 0 {
		return []string{convert(body)}
	}
	return result
}

func dialectSafeCuts(body []rune, format string) []int {
	if format != "html" {
		cuts := make([]int, len(body)+1)
		for i := range cuts {
			cuts[i] = i
		}
		return cuts
	}
	var cuts []int
	entityEnd := -1
	for i := 0; i < len(body); i++ {
		if i > entityEnd {
			cuts = append(cuts, i)
		}
		if body[i] == '<' {
			quote := rune(0)
			end := -1
			for j := i + 1; j < len(body); j++ {
				r := body[j]
				if quote != 0 {
					if r == quote {
						quote = 0
					}
					continue
				}
				if r == '\'' || r == '"' {
					quote = r
				} else if r == '>' {
					end = j
					break
				}
			}
			if end < 0 {
				break
			}
			i = end
		} else if body[i] == '&' {
			for j := i + 1; j < min(len(body), i+33); j++ {
				if unicode.IsSpace(body[j]) {
					break
				}
				if body[j] == ';' {
					entityEnd = j
					break
				}
			}
		}
	}
	return append(cuts, len(body))
}
func fitDialectCuts(body string, limit int, format string, truncate bool, convert func(string) string) []string {
	converted := convert(body)
	if body == "" || utf8.RuneCountInString(converted) <= limit {
		return []string{converted}
	}
	runes := []rune(body)
	cuts := dialectSafeCuts(runes, format)
	offset := 0
	var result []string
	for offset < len(runes) {
		first := sort.Search(len(cuts), func(i int) bool { return cuts[i] > offset })
		lo, hi := first, len(cuts)-1
		best := ""
		consumed := 0
		span := max(1, limit)
		for lo <= hi {
			probe := max(lo, sort.Search(len(cuts), func(i int) bool { return cuts[i] > offset+span })-1)
			probe = min(probe, hi)
			candidate := convert(string(runes[offset:cuts[probe]]))
			if utf8.RuneCountInString(candidate) > limit {
				hi = probe - 1
				break
			}
			best = candidate
			consumed = cuts[probe] - offset
			lo = probe + 1
			span *= 2
		}
		for lo <= hi {
			mid := (lo + hi) / 2
			candidate := convert(string(runes[offset:cuts[mid]]))
			if utf8.RuneCountInString(candidate) <= limit {
				best = candidate
				consumed = cuts[mid] - offset
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		if consumed == 0 {
			consumed = cuts[first] - offset
			best = convert(string(runes[offset : offset+consumed]))
		}
		if best != "" || len(result) == 0 {
			result = append(result, best)
		}
		offset += consumed
		if truncate {
			break
		}
	}
	return result
}
