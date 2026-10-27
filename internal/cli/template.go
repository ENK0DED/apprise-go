package cli

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var templateName = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)
var templateMarker = regexp.MustCompile(`\$\{([A-Za-z0-9_]{1,32})\}`)

func templateValue(name string, value any) (string, error) {
	var text string
	switch v := value.(type) {
	case string:
		text = v
	case bool:
		if v {
			text = "yes"
		} else {
			text = "no"
		}
	case int, int64, uint64, float64:
		text = fmt.Sprint(v)
	default:
		return "", fmt.Errorf("template variable '%s' must be text", name)
	}
	if utf8.RuneCountInString(text) > 1024 {
		return "", fmt.Errorf("template variable '%s' exceeds 1024 characters", name)
	}
	for _, r := range text {
		if r < 32 || r == 127 || r == 0x2028 || r == 0x2029 {
			return "", fmt.Errorf("template variable '%s' contains a control character", name)
		}
	}
	return text, nil
}
func loadTaggedURLsForOptions(opts *cliOptions, paths []string) []taggedURL {
	var result []taggedURL
	for _, path := range paths {
		path = expandPath(path)
		if !isYAMLConfigPath(path) {
			result = append(result, parseConfigFile(path)...)
			continue
		}
		data, err := os.ReadFile(path) // #nosec G304 -- explicit user-selected YAML configuration path
		if err != nil {
			continue
		}
		var document map[string]any
		if yaml.Unmarshal(data, &document) != nil {
			continue
		}
		declarations, exists := document["template"]
		if !exists {
			result = append(result, parseConfigFile(path)...)
			continue
		}
		defaults := map[string]any{}
		add := func(name string, value any) error {
			if !templateName.MatchString(name) {
				return fmt.Errorf("invalid template variable name")
			}
			key := strings.ToLower(name)
			if _, exists := defaults[key]; exists {
				return fmt.Errorf("template variable '%s' is declared more than once", key)
			}
			if value != nil {
				if _, err := templateValue(key, value); err != nil {
					return err
				}
			}
			defaults[key] = value
			return nil
		}
		var parseErr error
		switch entries := declarations.(type) {
		case map[string]any:
			for name, value := range entries {
				if err := add(name, value); err != nil {
					parseErr = err
					break
				}
			}
		case []any:
			for _, entry := range entries {
				switch entry := entry.(type) {
				case string:
					parseErr = add(entry, nil)
				case map[string]any:
					for name, value := range entry {
						if err := add(name, value); err != nil {
							parseErr = err
							break
						}
					}
				default:
					parseErr = fmt.Errorf("invalid template declaration entry")
				}
				if parseErr != nil {
					break
				}
			}
		default:
			parseErr = fmt.Errorf("the template section must be a list or a mapping")
		}
		if parseErr != nil {
			opts.templateErr = parseErr
			return nil
		}
		delete(document, "template")
		resolve := func(name string) (string, error) {
			key := strings.ToLower(name)
			if value := strings.TrimSpace(opts.templateValues[key]); value != "" {
				return templateValue(key, value)
			}
			if defaults[key] != nil {
				return templateValue(key, defaults[key])
			}
			if value := strings.TrimSpace(os.Getenv("APPRISE_TEMPLATE_" + strings.ToUpper(key))); value != "" {
				return templateValue(key, value)
			}
			return "", fmt.Errorf("no value available for template variable '%s'", key)
		}
		var rewrite func(any, bool) (any, error)
		rewrite = func(value any, tag bool) (any, error) {
			switch value := value.(type) {
			case string:
				return substituteTemplateText(value, tag, defaults, resolve)

			case []any:
				for i, item := range value {
					next, err := rewrite(item, tag)
					if err != nil {
						return nil, err
					}
					value[i] = next
				}
				return value, nil
			case map[string]any:
				next := map[string]any{}
				for key, item := range value {
					// YAML setting names stay literal. URL mapping keys are URLs.
					mapped := key
					if strings.Contains(key, "://") {
						resolved, err := rewrite(key, tag)
						if err != nil {
							return nil, err
						}
						mapped = resolved.(string)
					}
					content, err := rewrite(item, tag || key == "tag" || key == "tags")
					if err != nil {
						return nil, err
					}
					next[mapped] = content
				}
				return next, nil
			default:
				return value, nil
			}
		}
		rewritten, err := rewrite(document, false)
		if err != nil {
			opts.templateErr = err
			return nil
		}
		encoded, err := yaml.Marshal(rewritten)
		if err != nil {
			opts.templateErr = fmt.Errorf("could not resolve configuration template")
			return nil
		}
		cfg, valid := parseYAMLConfigWithStatus(encoded)
		if valid {
			result = append(result, applyTagGroups(cfg.URLs, cfg.Groups)...)
		}
	}
	return result
}

// Substitute only original marker positions. The URL schema and YAML setting
// names stay literal; values cannot create a new URL component accidentally.
func substituteTemplateText(value string, tag bool, defaults map[string]any, resolve func(string) (string, error)) (string, error) {
	separator := strings.Index(value, "://")
	authorityEnd := len(value)
	if separator >= 0 {
		if end := strings.IndexAny(value[separator+3:], "/?#"); end >= 0 {
			authorityEnd = separator + 3 + end
		}
	}
	query := strings.IndexByte(value, '?')
	var out strings.Builder
	previous := 0
	for _, match := range templateMarker.FindAllStringSubmatchIndex(value, -1) {
		start, end := match[0], match[1]
		name := value[match[2]:match[3]]
		if _, declared := defaults[strings.ToLower(name)]; !declared || separator >= 0 && start < separator+3 {
			continue
		}
		if tag {
			return "", fmt.Errorf("template variables are not allowed in tags")
		}
		replacement, err := resolve(name)
		if err != nil {
			return "", err
		}
		if separator >= 0 {
			switch {
			case start < authorityEnd:
				authority := value[separator+3 : authorityEnd]
				switch {
				case start == separator+3 && end == authorityEnd:
					// Upstream accepts user[:password]@host for a whole authority.
					if at := strings.LastIndexByte(replacement, '@'); at >= 0 {
						credentials, host := replacement[:at], replacement[at+1:]
						user, password, hasPassword := strings.Cut(credentials, ":")
						if hasPassword {
							replacement = url.UserPassword(user, password).String() + "@" + host
						} else {
							replacement = url.User(user).String() + "@" + host
						}
					}
					if parsed, err := url.Parse("http://" + replacement); err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
						return "", fmt.Errorf("invalid template authority")
					}
				case strings.LastIndexByte(authority, '@') >= 0 && start < separator+3+strings.LastIndexByte(authority, '@'):
					// A whole user field also accepts user:password upstream.
					if start == separator+3 && end == separator+3+strings.LastIndexByte(authority, '@') {
						if user, password, found := strings.Cut(replacement, ":"); found {
							replacement = url.UserPassword(user, password).String()
						} else {
							replacement = url.User(replacement).String()
						}
					} else {
						replacement = strings.ReplaceAll(url.QueryEscape(replacement), "+", "%20")
					}
				default:
					if strings.ContainsAny(replacement, "/@?#:") {
						return "", fmt.Errorf("invalid template hostname")
					}
				}
			case query >= 0 && start > query:
				replacement = url.QueryEscape(replacement)
			default:
				replacement = url.PathEscape(replacement)
			}
		}
		out.WriteString(value[previous:start])
		out.WriteString(replacement)
		previous = end
	}
	out.WriteString(value[previous:])
	return out.String(), nil
}
