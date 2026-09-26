package auth

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
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

// CurrentToken returns the currently stored token without refreshing it.
func (m *TokenManager) CurrentToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token
}

// GenerateQRCodePNG creates a PNG image of the current token QR code.
func (m *TokenManager) GenerateQRCodePNG() ([]byte, error) {
	token, err := m.GetToken()
	if err != nil {
		return nil, err
	}
	return qrcode.Encode(token, qrcode.Medium, 256)
}

// PrintTerminalQR prints an ANSI QR code of the current token to the given writer.
func (m *TokenManager) PrintTerminalQR(w io.Writer) error {
	token, err := m.GetToken()
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
	qrterminal.GenerateWithConfig(token, config)
	return nil
}

// GenerateRandomToken generates a cryptographically secure random alphanumeric string.
func GenerateRandomToken(length int) (string, error) {
	var sb strings.Builder
	sb.Grow(length)
	maxIdx := big.NewInt(int64(len(tokenCharset)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", err
		}
		sb.WriteByte(tokenCharset[n.Int64()])
	}

	return sb.String(), nil
}
