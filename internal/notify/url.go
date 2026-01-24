package notify

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ParsedURL struct {
	Raw          string
	Scheme       string
	Host         string
	Port         int
	User         string
	Password     string
	Path         string
	Query        map[string]string
	QueryAdd     map[string]string
	QueryDel     map[string]string
	QueryPayload map[string]string
}

func ParseURL(raw string) (*ParsedURL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty url")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" {
		return nil, fmt.Errorf("missing scheme")
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}

	port := 0
	if portRaw := u.Port(); portRaw != "" {
		value, err := strconv.Atoi(portRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", portRaw)
		}
		port = value
	}

	user := ""
	password := ""
	if u.User != nil {
		user = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			password = pw
		}
	}

	parsedPath := u.EscapedPath()
	if parsedPath == "." {
		parsedPath = ""
	}

	qsd := parseQSD(u.RawQuery, false, true)

	return &ParsedURL{
		Raw:          raw,
		Scheme:       strings.ToLower(u.Scheme),
		Host:         host,
		Port:         port,
		User:         user,
		Password:     password,
		Path:         parsedPath,
		Query:        qsd.qsd,
		QueryAdd:     qsd.add,
		QueryDel:     qsd.del,
		QueryPayload: qsd.payload,
	}, nil
}

type qsdResult struct {
	qsd     map[string]string
	add     map[string]string
	del     map[string]string
	payload map[string]string
}

func parseQSD(raw string, plusToSpace bool, sanitize bool) qsdResult {
	result := qsdResult{
		qsd:     map[string]string{},
		add:     map[string]string{},
		del:     map[string]string{},
		payload: map[string]string{},
	}

	if raw == "" {
		return result
	}

	pairs := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '&' || r == ';'
	})

	for _, pair := range pairs {
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		key := parts[0]
		val := ""
		if len(parts) == 2 {
			val = parts[1]
		}

		key = normalizeKey(key)
		key = decodeQueryValue(key)
		key = strings.TrimSpace(key)

		if plusToSpace {
			val = strings.ReplaceAll(val, "+", " ")
		}
		val = decodeQueryValue(val)
		val = strings.TrimSpace(val)

		storeKey := key
		if sanitize {
			storeKey = strings.ToLower(strings.TrimSpace(key))
		}
		result.qsd[storeKey] = val

		if strings.HasPrefix(key, "+") && len(key) > 1 {
			result.add[key[1:]] = val
		}
		if strings.HasPrefix(key, "-") && len(key) > 1 {
			result.del[key[1:]] = val
		}
		if strings.HasPrefix(key, ":") && len(key) > 1 {
			result.payload[key[1:]] = val
		}
	}

	return result
}

func normalizeKey(raw string) string {
	if raw == "" {
		return ""
	}

	first := raw[:1]
	rest := ""
	if len(raw) > 1 {
		rest = strings.ReplaceAll(raw[1:], "+", " ")
	}
	return first + rest
}

func decodeQueryValue(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}
