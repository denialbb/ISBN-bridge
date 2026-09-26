package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokenGenerationAndExpiration(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "token.txt")

	mgr := NewTokenManager(tokenFile, 50*time.Millisecond)

	token1, err := mgr.GetToken()
	if err != nil {
		t.Fatalf("unexpected error getting token: %v", err)
	}

	if len(token1) != 48 {
		t.Errorf("expected token length 48, got %d (%s)", len(token1), token1)
	}

	// Verify token was written to file
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("failed to read token file: %v", err)
	}
	if strings.TrimSpace(string(data)) != token1 {
		t.Errorf("file content %q != token %q", string(data), token1)
	}

	// Immediate next call should return same token
	token2, err := mgr.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token2 != token1 {
		t.Errorf("expected same token before expiry, got %s != %s", token2, token1)
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	if !mgr.IsExpired() {
		t.Error("expected token to be expired after 60ms")
	}

	// Getting token after expiry should refresh it
	token3, err := mgr.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token3 == token1 {
		t.Errorf("expected new token after expiry, got same token %s", token3)
	}
}

func TestVerifier(t *testing.T) {
	token := "XJHNiW2BsqPCyAl7h4TxZwLrMme9Yt3EUdK8bgO1zkQFc6Da"
	isbn := "9780306406157"
	now := time.Now().Format("2006-01-02 15:04:05")

	// Calculate correct SHA-256 for: ISBN|timestamp|token
	rawString := isbn + "|" + now + "|" + token
	hash := sha256.Sum256([]byte(rawString))
	validHash := hex.EncodeToString(hash[:])

	tests := []struct {
		name        string
		isbn        string
		timestamp   string
		authHeader  string
		token       string
		expectValid bool
	}{
		{
			name:        "valid hash with Bearer prefix",
			isbn:        isbn,
			timestamp:   now,
			authHeader:  "Bearer " + validHash,
			token:       token,
			expectValid: true,
		},
		{
			name:        "valid hash with uppercase hex",
			isbn:        isbn,
			timestamp:   now,
			authHeader:  "Bearer " + strings.ToUpper(validHash),
			token:       token,
			expectValid: true,
		},
		{
			name:        "missing Bearer prefix",
			isbn:        isbn,
			timestamp:   now,
			authHeader:  validHash,
			token:       token,
			expectValid: true, // Should gracefully support both with or without "Bearer "
		},
		{
			name:        "wrong token",
			isbn:        isbn,
			timestamp:   now,
			authHeader:  "Bearer " + validHash,
			token:       "wrongToken12345678901234567890123456789012345678",
			expectValid: false,
		},
		{
			name:        "tampered ISBN",
			isbn:        "9780241316757",
			timestamp:   now,
			authHeader:  "Bearer " + validHash,
			token:       token,
			expectValid: false,
		},
		{
			name:        "tampered timestamp",
			isbn:        isbn,
			timestamp:   "2026-09-01 00:00:00",
			authHeader:  "Bearer " + validHash,
			token:       token,
			expectValid: false,
		},
		{
			name:        "empty auth header",
			isbn:        isbn,
			timestamp:   now,
			authHeader:  "",
			token:       token,
			expectValid: false,
		},
	}

	verifier := NewVerifier(15 * time.Minute)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifier.Verify(tt.isbn, tt.timestamp, tt.authHeader, tt.token)
			if tt.expectValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.expectValid && err == nil {
				t.Errorf("expected error, got valid")
			}
		})
	}
}

func TestTimestampExpiry(t *testing.T) {
	token := "secretToken123"
	isbn := "9780306406157"
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	rawString := isbn + "|" + oldTime + "|" + token
	hash := sha256.Sum256([]byte(rawString))
	oldHash := hex.EncodeToString(hash[:])

	verifier := NewVerifier(15 * time.Minute)
	err := verifier.Verify(isbn, oldTime, "Bearer "+oldHash, token)
	if err == nil {
		t.Error("expected error for expired timestamp (replay attack), got nil")
	}
}
