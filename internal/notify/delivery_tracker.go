package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// deliveryTracker belongs to one logical notification. Each split piece has
// its own target set; a retry skips only targets that accepted that piece.
type deliveryTracker struct {
	delivered map[int]map[string]bool
	memo      map[int]map[string]string
	piece     int
}

func (t *deliveryTracker) beginNotification(retries bool) {
	t.delivered = nil
	t.memo = nil
	if retries {
		t.delivered = make(map[int]map[string]bool)
		t.memo = make(map[int]map[string]string)
	}
}

func (t *deliveryTracker) recall(key string) string { return t.memo[t.piece][key] }

func (t *deliveryTracker) remember(key, value string) {
	if t.memo == nil {
		return
	}
	if t.memo[t.piece] == nil {
		t.memo[t.piece] = make(map[string]string)
	}
	t.memo[t.piece][key] = value
}

func (t *deliveryTracker) beginPiece(index int) { t.piece = index }

func (t *deliveryTracker) isDelivered(key string) bool {
	return t.delivered[t.piece][key]
}

func (t *deliveryTracker) markDelivered(key string) {
	if t.delivered == nil {
		return
	}
	if t.delivered[t.piece] == nil {
		t.delivered[t.piece] = make(map[string]bool)
	}
	t.delivered[t.piece][key] = true
}

type notificationLifecycle interface {
	beginNotification(bool)
	beginPiece(int)
}

// Completed deliveries use the provider operation, its call arguments, and
// stable target positions. A failure before one request cannot shift another
// target's identity. Lookup calls and semantic acknowledgments stay local.
func (t *deliveryTracker) sendTrackedRequestFor(spec RequestSpec, identity ...any) error {
	if t.delivered == nil {
		return SendRequest(spec)
	}
	hash := sha256.New()
	if err := json.NewEncoder(hash).Encode(identity); err != nil {
		return fmt.Errorf("delivery identity: %w", err)
	}
	key := "http-delivery:" + hex.EncodeToString(hash.Sum(nil))
	if t.isDelivered(key) {
		return nil
	}
	err := SendRequest(spec)
	if err == nil {
		t.markDelivered(key)
	}
	return err
}
