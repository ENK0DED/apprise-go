package notify

import "strings"

type NotifyType string

const (
	NotifyInfo    NotifyType = "info"
	NotifySuccess NotifyType = "success"
	NotifyWarning NotifyType = "warning"
	NotifyFailure NotifyType = "failure"
)

func ParseNotifyType(raw string) (NotifyType, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch NotifyType(value) {
	case NotifyInfo, NotifySuccess, NotifyWarning, NotifyFailure:
		return NotifyType(value), true
	default:
		return NotifyInfo, false
	}
}
