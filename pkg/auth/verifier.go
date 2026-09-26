package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidAuthHeader  = errors.New("missing or malformed authorization header")
	ErrInvalidSignature   = errors.New("invalid SHA-256 signature")
	ErrTimestampExpired   = errors.New("timestamp expired or outside allowed time window")
	ErrEmptyTokenOrISBN   = errors.New("token and ISBN must not be empty")
)

// Common date formats produced by iOS Shortcuts Format Date action
var timestampFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"02/01/2006, 15:04:05",
	"02/01/2006 15:04:05",
	"02/01/2006 15:04",
	"02/01/06 15:04",
	"2006/01/02 15:04:05",
	"2006-01-02",
	time.RFC1123,
	time.RFC822,
}

// Verifier validates SHA-256 request signatures and timestamp freshness.
type Verifier struct {
	maxSkew time.Duration
}

// NewVerifier creates a new Verifier with a maximum acceptable timestamp skew.
func NewVerifier(maxSkew time.Duration) *Verifier {
	if maxSkew <= 0 {
		maxSkew = 15 * time.Minute
	}
	return &Verifier{maxSkew: maxSkew}
}

// Verify verifies the SHA-256 hash: sha256(isbn + "|" + timestamp + "|" + token).
func (v *Verifier) Verify(isbn, timestamp, authHeader, token string) error {
	if isbn == "" || token == "" {
		return ErrEmptyTokenOrISBN
	}

	receivedHash := extractBearerToken(authHeader)
	if receivedHash == "" {
		return ErrInvalidAuthHeader
	}

	// 1. Verify timestamp freshness if timestamp string is provided
	if timestamp != "" {
		if t, ok := parseTimestamp(timestamp); ok {
			diff := time.Since(t)
			if diff < 0 {
				diff = -diff
			}
			if diff > v.maxSkew {
				return fmt.Errorf("%w: difference is %v (max allowed: %v)", ErrTimestampExpired, diff, v.maxSkew)
			}
		}
	}

	// 2. Compute expected SHA-256 hash: ISBN|timestamp|token
	raw := isbn + "|" + timestamp + "|" + token
	sum := sha256.Sum256([]byte(raw))
	expectedHash := hex.EncodeToString(sum[:])

	// Case-insensitive constant-time comparison
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(expectedHash)), []byte(strings.ToLower(receivedHash))) != 1 {
		return ErrInvalidSignature
	}

	return nil
}

func extractBearerToken(authHeader string) string {
	trimmed := strings.TrimSpace(authHeader)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}

func parseTimestamp(val string) (time.Time, bool) {
	val = strings.TrimSpace(val)

	// Try numeric unix timestamp (seconds or milliseconds)
	if n, err := strconv.ParseInt(val, 10, 64); err == nil {
		if n > 1e11 { // milliseconds
			return time.UnixMilli(n), true
		}
		return time.Unix(n, 0), true
	}

	for _, format := range timestampFormats {
		if t, err := time.ParseInLocation(format, val, time.Local); err == nil {
			return t, true
		}
		if t, err := time.Parse(format, val); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}
