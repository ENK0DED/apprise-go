package notify

import "strings"

func ConvertMessageFormatForTarget(parsed *ParsedURL, content, inputFormat string) (string, error) {
	// Expand before text escaping can alter codes such as :thumbs_up:.
	if parsed != nil && parseBoolWithDefault(parsed.Query["emojis"], false) {
		content = ApplyEmojis(content)
	}
	outputFormat := resolveTargetFormat(parsed, inputFormat)
	if parsed != nil {
		parsed.resolvedNotifyFormat = outputFormat
		parsed.sourceNotifyFormat = normalizeNotifyFormat(inputFormat)
	}
	// An omitted source format is upstream's explicit passthrough mode.
	if strings.TrimSpace(inputFormat) == "" {
		return content, nil
	}

	return ConvertMessageFormat(content, inputFormat, outputFormat)
}

// resolveTargetFormat preserves upstream's declaration order. Single-format
// services keep their historical URL override; multi-format services prefer a
// supported override, then the caller's format, then the first declaration.
func resolveTargetFormat(parsed *ParsedURL, inputFormat string) string {
	if parsed == nil {
		return ""
	}
	if parsed.Scheme == "fluxer" || parsed.Scheme == "fluxers" {
		for _, key := range []string{"href", "url", "thread"} {
			if _, exists := parsed.Query[key]; exists {
				return "markdown"
			}
		}
	}
	if parsed.Scheme == "revolt" {
		if _, explicit := parsed.Query["format"]; !explicit {
			return "markdown"
		}
	}
	if parsed.Scheme == "deltachat" || parsed.Scheme == "deltachats" {
		return "text"
	}
	// The Apprise API relays the caller's format to the receiving server.
	if (parsed.Scheme == "apprise" || parsed.Scheme == "apprises") && isSupportedNotifyFormat(normalizeNotifyFormat(inputFormat)) {
		return normalizeNotifyFormat(inputFormat)
	}
	entry, ok := SchemaEntryForSchema(parsed.Scheme)
	if !ok {
		return ""
	}
	details, _ := entry["details"].(map[string]any)
	args, _ := details["args"].(map[string]any)
	spec, _ := args["format"].(map[string]any)
	formats := extractSchemaList(SchemaEntry(spec), "supported")
	if len(formats) == 0 {
		return OverflowLimitsFor(parsed.Scheme).Format
	}
	if len(formats) == 1 {
		if requested := normalizeNotifyFormat(parsed.Query["format"]); requested != "" {
			return requested
		}
		return formats[0]
	}
	for _, candidate := range []string{normalizeNotifyFormat(parsed.Query["format"]), normalizeNotifyFormat(inputFormat)} {
		for _, supported := range formats {
			if candidate == supported {
				return supported
			}
		}
	}
	return formats[0]
}

func targetNotifyFormat(parsed *ParsedURL) string {
	if parsed.resolvedNotifyFormat != "" {
		return parsed.resolvedNotifyFormat
	}
	return resolveTargetFormat(parsed, "")
}
