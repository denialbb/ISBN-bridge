package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image/color"
	"image/png"
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
		{
			name:        "missing timestamp",
			isbn:        isbn,
			timestamp:   "",
			authHeader:  "Bearer " + validHash,
			token:       token,
			expectValid: false,
		},
		{
			name:        "unparseable timestamp",
			isbn:        isbn,
			timestamp:   "not-a-date",
			authHeader:  "Bearer " + validHash,
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

func TestPairingPayload(t *testing.T) {
	tempDir := t.TempDir()
	tokenFile := filepath.Join(tempDir, "token.txt")
	mgr := NewTokenManager(tokenFile, time.Hour)

	token, err := mgr.GetToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without baseURL: returns raw token
	payload, err := mgr.GetPairingPayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload != token {
		t.Errorf("expected raw token %q, got %q", token, payload)
	}

	// With baseURL: returns full pairing URL
	mgr.SetBaseURL("http://192.168.1.107:8765")
	if mgr.BaseURL() != "http://192.168.1.107:8765" {
		t.Errorf("expected base URL http://192.168.1.107:8765, got %q", mgr.BaseURL())
	}

	payload, err = mgr.GetPairingPayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedURL := "http://192.168.1.107:8765/pair?token=" + token
	if payload != expectedURL {
		t.Errorf("expected pairing URL %q, got %q", expectedURL, payload)
	}

	// Generate PNG should succeed
	png, err := mgr.GenerateQRCodePNG()
	if err != nil || len(png) == 0 {
		t.Errorf("failed to generate pairing QR PNG: %v", err)
	}

	// Dynamic URL resolver overrides static baseURL
	mgr.SetURLResolver(func() string {
		return "http://172.20.10.2:8765"
	})
	if mgr.BaseURL() != "http://172.20.10.2:8765" {
		t.Errorf("expected dynamic resolver URL, got %q", mgr.BaseURL())
	}
	payload, err = mgr.GetPairingPayload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedUSBURL := "http://172.20.10.2:8765/pair?token=" + token
	if payload != expectedUSBURL {
		t.Errorf("expected dynamic pairing URL %q, got %q", expectedUSBURL, payload)
	}
}

func TestQRRecolorBrandInk(t *testing.T) {
	mgr := NewTokenManager(filepath.Join(t.TempDir(), "token.txt"), time.Hour)
	mgr.SetBaseURL("http://192.168.1.107:8765")
	out, err := mgr.GenerateQRCodePNG()
	if err != nil {
		t.Fatalf("failed to generate QR: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("generated QR is not a valid PNG: %v", err)
	}
	bounds := img.Bounds()
	var navy, black, white int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			switch {
			case r>>8 == 0x2F && g>>8 == 0x4A && b>>8 == 0x6E:
				navy++
			case r < 0x8000 && g < 0x8000 && b < 0x8000:
				black++
			default:
				white++
			}
		}
	}
	if navy == 0 {
		t.Error("expected brand-ink modules in recolored QR, found none")
	}
	if black != 0 {
		t.Errorf("expected no pure-black modules after recolor, found %d", black)
	}
	if white == 0 {
		t.Error("expected white background to survive recoloring")
	}
}

func TestParseHexColor(t *testing.T) {
	fb := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	cases := []struct {
		in   string
		want color.RGBA
	}{
		{"3CBF5C", color.RGBA{R: 0x3C, G: 0xBF, B: 0x5C, A: 255}},
		{"#080C09", color.RGBA{R: 0x08, G: 0x0C, B: 0x09, A: 255}},
		{"fff", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"  2F4A6E  ", color.RGBA{R: 0x2F, G: 0x4A, B: 0x6E, A: 255}},
		{"red", fb},
		{"12345", fb},
		{"1234567", fb},
		{"ZZZZZZ", fb},
		{"", fb},
	}
	for _, tc := range cases {
		if got := ParseHexColor(tc.in, fb); got != tc.want {
			t.Errorf("ParseHexColor(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestGenerateQRCodePNGWithColors(t *testing.T) {
	mgr := NewTokenManager(filepath.Join(t.TempDir(), "token.txt"), time.Hour)
	mgr.SetBaseURL("http://192.168.1.107:8765")
	ink := color.RGBA{R: 0x3C, G: 0xBF, B: 0x5C, A: 255}
	bg := color.RGBA{R: 0x08, G: 0x0C, B: 0x09, A: 255}
	out, err := mgr.GenerateQRCodePNGWithColors(ink, bg)
	if err != nil {
		t.Fatalf("failed to generate themed QR: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("themed QR is not a valid PNG: %v", err)
	}
	bounds := img.Bounds()
	var accent, other int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			switch {
			case r>>8 == 0x3C && g>>8 == 0xBF && b>>8 == 0x5C:
				accent++
			case r>>8 == 0x08 && g>>8 == 0x0C && b>>8 == 0x09:
				// themed background
			default:
				other++
			}
		}
	}
	if accent == 0 {
		t.Error("expected accent modules in themed QR, found none")
	}
	if other != 0 {
		t.Errorf("expected only accent/background pixels, found %d others", other)
	}
}
