package notify

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func retrySettings(parsed *ParsedURL) (int, time.Duration, error) {
	if parsed == nil {
		return 0, 0, nil
	}
	retries := 0
	raw := strings.TrimSpace(parsed.Query["retry"])
	if value, err := strconv.Atoi(raw); err == nil {
		if value < 0 {
			return 0, 0, fmt.Errorf("service retry count must be nonnegative")
		}
		retries = min(value, 10)
	} else if raw != "" {
		digits := strings.TrimPrefix(strings.TrimPrefix(raw, "+"), "-")
		valid := digits != ""
		for _, r := range digits {
			if r < '0' || r > '9' {
				valid = false
				break
			}
		}
		if valid {
			if strings.HasPrefix(raw, "-") {
				return 0, 0, fmt.Errorf("service retry count must be nonnegative")
			}
			retries = 10
		}
	}
	seconds := 0.0
	raw = strings.TrimSpace(parsed.Query["wait"])
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return 0, 0, fmt.Errorf("service retry wait must be finite and nonnegative")
		}
		seconds = math.Min(value, 20)
	} else if strings.EqualFold(strings.TrimLeft(raw, "+-"), "nan") {
		return 0, 0, fmt.Errorf("service retry wait must be finite and nonnegative")
	}
	return retries, time.Duration(seconds * float64(time.Second)), nil
}
