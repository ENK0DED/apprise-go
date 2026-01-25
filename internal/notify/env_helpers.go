package notify

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

func fixedTime() time.Time {
	raw := strings.TrimSpace(os.Getenv("APPRISE_FIXED_TIME"))
	if raw == "" {
		return time.Now().UTC()
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Now().UTC()
	}

	return parsed.UTC()
}

func oauthTimestamp() string {
	raw := strings.TrimSpace(os.Getenv("APPRISE_OAUTH_TIMESTAMP"))
	if raw != "" {
		return raw
	}
	return strconv.FormatInt(fixedTime().Unix(), 10)
}

func oauthNonce() string {
	raw := strings.TrimSpace(os.Getenv("APPRISE_OAUTH_NONCE"))
	if raw != "" {
		return raw
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}
