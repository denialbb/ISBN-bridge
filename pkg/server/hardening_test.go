package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestReplayCache_SingleUseAndExpiry(t *testing.T) {
	c := newReplayCache(10, 50*time.Millisecond)
	now := time.Now()

	if c.checkAndMark("sig1", now) {
		t.Error("expected first use to be fresh")
	}
	if !c.checkAndMark("sig1", now) {
		t.Error("expected immediate reuse to be a replay")
	}
	if c.checkAndMark("sig2", now) {
		t.Error("expected different signature to be fresh")
	}

	// After TTL expiry the signature is usable again (bounded memory
	// means the cache is not a permanent blocklist).
	if c.checkAndMark("sig1", now.Add(100*time.Millisecond)) {
		t.Error("expected expired signature to be fresh again")
	}
}

func TestReplayCache_Bounded(t *testing.T) {
	c := newReplayCache(5, time.Minute)
	now := time.Now()
	for i := 0; i < 50; i++ {
		c.checkAndMark("sig", now)
	}
	c.mu.Lock()
	n := len(c.seen)
	c.mu.Unlock()
	if n > 5 {
		t.Errorf("expected cache bounded to 5 entries, got %d", n)
	}
}

func TestRateLimiter_Window(t *testing.T) {
	l := newRateLimiter(2, 10*time.Second)
	now := time.Now()
	l.nowFunc = func() time.Time { return now }

	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") {
		t.Fatal("expected first two requests to pass")
	}
	if l.allow("1.2.3.4") {
		t.Error("expected third request within window to be rejected")
	}
	// Other IPs get their own budget.
	if !l.allow("5.6.7.8") {
		t.Error("expected different IP to have its own budget")
	}

	// After the window slides, budget is restored.
	now = now.Add(11 * time.Second)
	if !l.allow("1.2.3.4") {
		t.Error("expected budget restored after window expiry")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	l := newRateLimiter(0, 10*time.Second)
	for i := 0; i < 100; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("expected disabled limiter to always allow (iteration %d)", i)
		}
	}
}

func TestNormalizeSignature(t *testing.T) {
	if got := normalizeSignature("Bearer AbC123 "); got != "abc123" {
		t.Errorf("expected canonical signature, got %q", got)
	}
	if got := normalizeSignature("abc123"); got != "abc123" {
		t.Errorf("expected prefix-less signature kept, got %q", got)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/isbn", nil)
	req.RemoteAddr = "192.168.1.50:54321"
	if got := clientIP(req); got != "192.168.1.50" {
		t.Errorf("expected host without port, got %q", got)
	}
}
