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
	mu        sync.RWMutex
	token     string
	createdAt time.Time
	ttl       time.Duration
	filePath  string
	baseURL   string
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

// BaseURL returns the configured base URL.
func (m *TokenManager) BaseURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseURL
}

// GetPairingPayload returns the pairing URL (http://<base>/pair?token=<token>) or raw token.
func (m *TokenManager) GetPairingPayload() (string, error) {
	token, err := m.GetToken()
	if err != nil {
		return "", err
	}
	m.mu.RLock()
	base := m.baseURL
	m.mu.RUnlock()

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

// GenerateQRCodePNG creates a PNG image of the current pairing QR code.
// The QR border is disabled so the popup card hugs the code; the card's
// own white margin doubles as the scanner quiet zone.
func (m *TokenManager) GenerateQRCodePNG() ([]byte, error) {
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
	return recolorQRPNG(raw, qrModuleInk)
}

// recolorQRPNG repaints dark QR modules with ink while leaving the white
// background and quiet zone untouched.
func recolorQRPNG(raw []byte, ink color.RGBA) ([]byte, error) {
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
				out.SetRGBA(x, y, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
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
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	charLen := byte(len(tokenCharset))
	for i := 0; i < length; i++ {
		bytes[i] = tokenCharset[bytes[i]%charLen]
	}
	return string(bytes), nil
}
