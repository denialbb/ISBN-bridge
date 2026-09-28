package auth

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mdp/qrterminal/v3"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	DefaultTokenLength = 48
	tokenCharset       = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

// TokenManager manages generating, persisting, rotating, and rendering QR codes for tokens.
type TokenManager struct {
	mu          sync.RWMutex
	token       string
	createdAt   time.Time
	ttl         time.Duration
	filePath    string
	baseURL     string
	urlResolver func() string
}

// NewTokenManager initializes a TokenManager with a persistence file path and token TTL.
func NewTokenManager(filePath string, ttl time.Duration) *TokenManager {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &TokenManager{
		filePath: filePath,
		ttl:      ttl,
	}
}

// GetToken returns the active token. If expired or empty, it automatically refreshes it.
func (m *TokenManager) GetToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token != "" && time.Since(m.createdAt) < m.ttl {
		return m.token, nil
	}

	return m.refreshLocked()
}

// RefreshToken manually forces token rotation.
func (m *TokenManager) RefreshToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLocked()
}

func (m *TokenManager) refreshLocked() (string, error) {
	newToken, err := GenerateRandomToken(DefaultTokenLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	m.token = newToken
	m.createdAt = time.Now()

	// Persist to file if filePath is provided
	if m.filePath != "" {
		dir := filepath.Dir(m.filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory for token: %w", err)
		}
		if err := os.WriteFile(m.filePath, []byte(newToken), 0600); err != nil {
			return "", fmt.Errorf("failed to write token to file: %w", err)
		}
	}

	return newToken, nil
}

// IsExpired reports whether the current token has exceeded its TTL.
func (m *TokenManager) IsExpired() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.token == "" {
		return true
	}
	return time.Since(m.createdAt) >= m.ttl
}

// SetTTL updates the token lifespan dynamically.
func (m *TokenManager) SetTTL(ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ttl > 0 {
		m.ttl = ttl
	}
}

// TTL returns the current token lifespan.
func (m *TokenManager) TTL() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ttl
}

// SetBaseURL configures the base URL (e.g. http://192.168.1.107:8765) for QR pairing.
func (m *TokenManager) SetBaseURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseURL = strings.TrimSuffix(url, "/")
}

// SetURLResolver configures a dynamic callback function to resolve the latest server base URL.
func (m *TokenManager) SetURLResolver(resolver func() string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urlResolver = resolver
}

// BaseURL returns the configured base URL (or resolves it dynamically if a resolver is registered).
func (m *TokenManager) BaseURL() string {
	m.mu.RLock()
	resolver := m.urlResolver
	base := m.baseURL
	m.mu.RUnlock()

	if resolver != nil {
		if resolved := resolver(); resolved != "" {
			return strings.TrimSuffix(resolved, "/")
		}
	}
	return base
}

// GetPairingPayload returns the pairing URL (http://<base>/pair?token=<token>) or raw token.
func (m *TokenManager) GetPairingPayload() (string, error) {
	token, err := m.GetToken()
	if err != nil {
		return "", err
	}
	base := m.BaseURL()

	if base == "" {
		return token, nil
	}
	return fmt.Sprintf("%s/pair?token=%s", base, token), nil
}

// CurrentToken returns the currently stored token without refreshing it.
func (m *TokenManager) CurrentToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token
}

// qrModuleInk is the brand ink used for QR modules. Dark navy keeps
// scanner contrast high on the white background and quiet zone.
var qrModuleInk = color.RGBA{R: 0x2F, G: 0x4A, B: 0x6E, A: 0xFF}

// qrPaper is the default QR background. The QR border is disabled so the
// popup card hugs the code; the card's own margin doubles as the quiet zone.
var qrPaper = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

// QRModuleInk returns the default brand ink for QR modules.
func QRModuleInk() color.RGBA { return qrModuleInk }

// QRPaper returns the default QR background.
func QRPaper() color.RGBA { return qrPaper }

// ParseHexColor parses "#RRGGBB" or "RRGGBB" (3-digit form accepted),
// returning fallback for anything else. Used for /qr.png ink/bg params so
// desktop clients can request themed (e.g. negative) QR codes.
func ParseHexColor(s string, fallback color.RGBA) color.RGBA {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	if len(s) == 3 {
		expanded := make([]byte, 0, 6)
		for i := 0; i < 3; i++ {
			expanded = append(expanded, s[i], s[i])
		}
		s = string(expanded)
	}
	if len(s) != 6 {
		return fallback
	}
	var v [3]uint8
	for i := 0; i < 3; i++ {
		n := hexVal(s[2*i])<<4 | hexVal(s[2*i+1])
		if n < 0 {
			return fallback
		}
		v[i] = uint8(n)
	}
	return color.RGBA{R: v[0], G: v[1], B: v[2], A: 0xFF}
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// GenerateQRCodePNG creates a PNG image of the current pairing QR code.
// The QR border is disabled so the popup card hugs the code; the card's
// own margin doubles as the scanner quiet zone.
func (m *TokenManager) GenerateQRCodePNG() ([]byte, error) {
	return m.GenerateQRCodePNGWithColors(qrModuleInk, qrPaper)
}

// GenerateQRCodePNGWithColors creates a PNG image of the current pairing
// QR code with custom module ink and background (e.g. a negative,
// theme-accented card for dark desktop themes).
func (m *TokenManager) GenerateQRCodePNGWithColors(ink, bg color.RGBA) ([]byte, error) {
	payload, err := m.GetPairingPayload()
	if err != nil {
		return nil, err
	}
	q, err := qrcode.New(payload, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	q.DisableBorder = true
	raw, err := q.PNG(256)
	if err != nil {
		return nil, err
	}
	return recolorQRPNG(raw, ink, bg)
}

// recolorQRPNG repaints dark QR modules with ink on bg, leaving the
// uniform background (and quiet zone) untouched.
func recolorQRPNG(raw []byte, ink, bg color.RGBA) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// r/g/b are 16-bit; luminance threshold at ~50% separates
			// near-black modules from the white background.
			lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
			if lum < 0.5*0xFFFF {
				out.SetRGBA(x, y, ink)
			} else {
				out.SetRGBA(x, y, bg)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PrintTerminalQR prints an ANSI QR code of the current pairing payload to the given writer.
func (m *TokenManager) PrintTerminalQR(w io.Writer) error {
	payload, err := m.GetPairingPayload()
	if err != nil {
		return err
	}

	config := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    w,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	}
	qrterminal.GenerateWithConfig(payload, config)
	return nil
}

func GenerateRandomToken(length int) (string, error) {
	// Rejection sampling avoids modulo bias: 256 is not a multiple of
	// the 62-character alphabet size, so plain `byte % 62` would favor
	// the first few characters.
	maxUnbiased := byte(256 - (256 % len(tokenCharset)))
	out := make([]byte, 0, length)
	chunk := make([]byte, length)
	for len(out) < length {
		if _, err := rand.Read(chunk); err != nil {
			return "", err
		}
		for _, b := range chunk {
			if b >= maxUnbiased {
				continue
			}
			out = append(out, tokenCharset[b%byte(len(tokenCharset))])
			if len(out) == length {
				break
			}
		}
	}
	return string(out), nil
}
