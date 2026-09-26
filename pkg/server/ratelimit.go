package server

import (
	"sync"
	"time"
)

// rateLimiter enforces a per-IP sliding-window request budget for the
// sensitive POST /isbn endpoint. It is an in-memory LAN anti-spam
// measure, not a distributed defense: state is local to this process.
type rateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	max     int
	window  time.Duration
	maxIPs  int
	nowFunc func() time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	if window <= 0 {
		window = 10 * time.Second
	}
	return &rateLimiter{
		hits:    make(map[string][]time.Time),
		max:     max,
		window:  window,
		maxIPs:  256,
		nowFunc: time.Now,
	}
}

// allow records a request from ip and reports whether it fits the budget.
// A non-positive max disables limiting (always allows).
func (l *rateLimiter) allow(ip string) bool {
	if l.max <= 0 {
		return true
	}
	now := l.nowFunc()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	recent := l.hits[ip][:0]
	for _, t := range l.hits[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= l.max {
		l.hits[ip] = recent
		return false
	}

	l.hits[ip] = append(recent, now)

	if len(l.hits) > l.maxIPs {
		// Bound memory when facing many distinct source IPs:
		// drop one arbitrary bucket (its budget simply resets).
		for k := range l.hits {
			delete(l.hits, k)
			break
		}
	}
	return true
}
