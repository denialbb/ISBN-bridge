package server

import (
	"sync"
	"time"
)

// replayCache remembers recently accepted request signatures so each one
// can be used exactly once. It is a purely in-memory, best-effort LAN
// hardening measure: entries expire after ttl and the table is bounded to
// maxEntries (oldest expiry evicted first when full).
type replayCache struct {
	mu         sync.Mutex
	seen       map[string]time.Time
	ttl        time.Duration
	maxEntries int
}

func newReplayCache(maxEntries int, ttl time.Duration) *replayCache {
	if maxEntries <= 0 {
		maxEntries = 100
	}
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &replayCache{
		seen:       make(map[string]time.Time, maxEntries),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// checkAndMark reports whether sig was already used within its TTL.
// A fresh signature is recorded and reports false; a remembered,
// unexpired one reports true (replay). Expired entries are treated as
// fresh and pruned lazily.
func (c *replayCache) checkAndMark(sig string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if expiry, ok := c.seen[sig]; ok {
		if now.Before(expiry) {
			return true
		}
		delete(c.seen, sig)
	}

	if len(c.seen) >= c.maxEntries {
		c.pruneLocked(now)
	}
	if len(c.seen) >= c.maxEntries {
		// Still full (clock skew edge): evict one arbitrary entry
		// rather than growing unboundedly.
		for k := range c.seen {
			delete(c.seen, k)
			break
		}
	}
	c.seen[sig] = now.Add(c.ttl)
	return false
}

func (c *replayCache) pruneLocked(now time.Time) {
	for k, expiry := range c.seen {
		if !now.Before(expiry) {
			delete(c.seen, k)
		}
	}
}
