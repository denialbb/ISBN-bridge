package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/denialbb/biblios-scanner/pkg/auth"
)

type mockForwarder struct {
	mu            sync.Mutex
	forwardedISBN string
	forwardCount  int
}

func (m *mockForwarder) Forward(ctx context.Context, isbn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forwardedISBN = isbn
	m.forwardCount++
	return nil
}

func setupTestServer(t *testing.T) (*Server, *auth.TokenManager, *mockForwarder) {
	tempDir := t.TempDir()
	tokenFile := tempDir + "/token.txt"
	tokenMgr := auth.NewTokenManager(tokenFile, 1*time.Hour)
	verifier := auth.NewVerifier(15 * time.Minute)
	forwarder := &mockForwarder{}

	srv := NewServer(Config{
		TokenManager: tokenMgr,
		Verifier:     verifier,
		Forwarder:    forwarder,
	})

	return srv, tokenMgr, forwarder
}

func TestPostISBN_Success(t *testing.T) {
	srv, tokenMgr, forwarder := setupTestServer(t)

	token, err := tokenMgr.GetToken()
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	rawISBN := "978-0-306-40615-7"
	normalizedISBN := "9780306406157"
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Calculate SHA256(rawISBN + "|" + timestamp + "|" + token)
	sum := sha256.Sum256([]byte(rawISBN + "|" + timestamp + "|" + token))
	hashHex := hex.EncodeToString(sum[:])

	req := httptest.NewRequest("POST", "/isbn", strings.NewReader(rawISBN))
	req.Header.Set("Authorization", "Bearer "+hashHex)
	req.Header.Set("Timestamp", timestamp)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got %q", rec.Body.String())
	}

	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	if forwarder.forwardCount != 1 {
		t.Errorf("expected forwardCount 1, got %d", forwarder.forwardCount)
	}
	if forwarder.forwardedISBN != normalizedISBN {
		t.Errorf("expected forwarded ISBN %q, got %q", normalizedISBN, forwarder.forwardedISBN)
	}
}

func TestPostISBN_InvalidSignature(t *testing.T) {
	srv, _, forwarder := setupTestServer(t)

	rawISBN := "9780306406157"
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	req := httptest.NewRequest("POST", "/isbn", strings.NewReader(rawISBN))
	req.Header.Set("Authorization", "Bearer invalidhash123")
	req.Header.Set("Timestamp", timestamp)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	if forwarder.forwardCount != 0 {
		t.Errorf("expected no forwarding on invalid signature, got %d", forwarder.forwardCount)
	}
}

func TestPostISBN_InvalidISBN(t *testing.T) {
	srv, tokenMgr, _ := setupTestServer(t)

	token, _ := tokenMgr.GetToken()
	badISBN := "12345"
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	sum := sha256.Sum256([]byte(badISBN + "|" + timestamp + "|" + token))
	hashHex := hex.EncodeToString(sum[:])

	req := httptest.NewRequest("POST", "/isbn", strings.NewReader(badISBN))
	req.Header.Set("Authorization", "Bearer "+hashHex)
	req.Header.Set("Timestamp", timestamp)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rec.Code)
	}
}

func TestGetQRCodePNG(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/qr.png", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected image/png, got %s", rec.Header().Get("Content-Type"))
	}

	body, _ := io.ReadAll(rec.Body)
	if len(body) < 100 {
		t.Errorf("expected non-trivial PNG payload, got %d bytes", len(body))
	}
}

func TestGetQRHtml(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/qr", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rec.Code)
	}

	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html, got %s", rec.Header().Get("Content-Type"))
	}

	if !strings.Contains(rec.Body.String(), "<img") {
		t.Errorf("expected HTML to contain QR image tag")
	}
}
