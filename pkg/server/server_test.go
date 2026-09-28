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

	"github.com/denialbb/isbn-bridge/pkg/auth"
)

type mockForwarder struct {
	mu            sync.Mutex
	forwardedISBN string
	forwardCount  int
	showQRCount   int
	hideQRCount   int
}

func (m *mockForwarder) Forward(ctx context.Context, isbn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.forwardedISBN = isbn
	m.forwardCount++
	return nil
}

func (m *mockForwarder) ShowQR(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.showQRCount++
	return nil
}

func (m *mockForwarder) HideQR(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hideQRCount++
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

func TestPostISBN_MissingTimestamp(t *testing.T) {
	srv, tokenMgr, forwarder := setupTestServer(t)

	token, _ := tokenMgr.GetToken()
	rawISBN := "9780306406157"
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	sum := sha256.Sum256([]byte(rawISBN + "|" + timestamp + "|" + token))
	hashHex := hex.EncodeToString(sum[:])

	req := httptest.NewRequest("POST", "/isbn", strings.NewReader(rawISBN))
	req.Header.Set("Authorization", "Bearer "+hashHex)
	// No Timestamp header: must be rejected, never silently accepted.

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing timestamp, got %d", rec.Code)
	}
	if forwarder.forwardCount != 0 {
		t.Errorf("expected no forwarding without timestamp, got %d", forwarder.forwardCount)
	}
}

func TestPostISBN_ReplayRejected(t *testing.T) {
	srv, tokenMgr, forwarder := setupTestServer(t)

	token, _ := tokenMgr.GetToken()
	rawISBN := "9780306406157"
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	sum := sha256.Sum256([]byte(rawISBN + "|" + timestamp + "|" + token))
	hashHex := hex.EncodeToString(sum[:])

	send := func() int {
		req := httptest.NewRequest("POST", "/isbn", strings.NewReader(rawISBN))
		req.Header.Set("Authorization", "Bearer "+hashHex)
		req.Header.Set("Timestamp", timestamp)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := send(); code != http.StatusOK {
		t.Fatalf("expected first use 200 OK, got %d", code)
	}
	if code := send(); code != http.StatusConflict {
		t.Errorf("expected replay 409 Conflict, got %d", code)
	}

	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	if forwarder.forwardCount != 1 {
		t.Errorf("expected exactly 1 forward (replay must not forward), got %d", forwarder.forwardCount)
	}
}

func TestPostISBN_RateLimited(t *testing.T) {
	srv, tokenMgr, _ := setupTestServer(t)

	token, _ := tokenMgr.GetToken()
	// Default budget without AppConfig: 2 requests per 10s per IP.
	// Distinct timestamps yield distinct signatures so only the
	// limiter (not the replay cache) can reject the third request.
	var codes []int
	for i := 0; i < 3; i++ {
		rawISBN := "9780306406157"
		timestamp := time.Now().Add(time.Duration(i) * time.Second).Format("2006-01-02 15:04:05")
		sum := sha256.Sum256([]byte(rawISBN + "|" + timestamp + "|" + token))
		hashHex := hex.EncodeToString(sum[:])

		req := httptest.NewRequest("POST", "/isbn", strings.NewReader(rawISBN))
		req.Header.Set("Authorization", "Bearer "+hashHex)
		req.Header.Set("Timestamp", timestamp)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
		t.Errorf("expected first two requests 200 OK, got %v", codes)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Errorf("expected third request 429 Too Many Requests, got %v", codes)
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

func TestPairEndpoint(t *testing.T) {
	srv, tokenMgr, _ := setupTestServer(t)
	token, err := tokenMgr.GetToken()
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	// 1. Unauthorized non-loopback access without token must return 401
	reqUnauth := httptest.NewRequest("GET", "/pair?format=json", nil)
	recUnauth := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated LAN request, got %d", recUnauth.Code)
	}

	// 2. Loopback access without token (localhost) must succeed with 200
	reqLocal := httptest.NewRequest("GET", "/pair?format=json", nil)
	reqLocal.RemoteAddr = "127.0.0.1:54321"
	recLocal := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recLocal, reqLocal)
	if recLocal.Code != http.StatusOK {
		t.Errorf("expected 200 for loopback request, got %d", recLocal.Code)
	}
	if !strings.Contains(recLocal.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected application/json, got %s", recLocal.Header().Get("Content-Type"))
	}
	if !strings.Contains(recLocal.Body.String(), `"url"`) || !strings.Contains(recLocal.Body.String(), `"token"`) {
		t.Errorf("expected JSON to contain url and token: %s", recLocal.Body.String())
	}

	// 3. Authenticated pairing with token parameter must succeed
	reqToken := httptest.NewRequest("GET", "/pair?format=json&token="+token, nil)
	recToken := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recToken, reqToken)
	if recToken.Code != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", recToken.Code)
	}
	if !strings.Contains(recToken.Body.String(), `"interface_type"`) {
		t.Errorf("expected JSON to contain interface_type: %s", recToken.Body.String())
	}

	// 3b. USB interface detection in pairing payload
	tokenMgr.SetBaseURL("http://172.20.10.2:8765")
	reqUSB := httptest.NewRequest("GET", "/pair?format=json&token="+token, nil)
	recUSB := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recUSB, reqUSB)
	if !strings.Contains(recUSB.Body.String(), `"interface_type":"usb_ios"`) {
		t.Errorf("expected interface_type usb_ios, got %s", recUSB.Body.String())
	}

	// 4. iOS HTML response with valid token
	reqIOS := httptest.NewRequest("GET", "/pair?token="+token, nil)
	reqIOS.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")
	recIOS := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recIOS, reqIOS)

	if recIOS.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", recIOS.Code)
	}
	if !strings.Contains(recIOS.Body.String(), "shortcuts://run-shortcut") {
		t.Errorf("expected iOS page to include shortcuts URI")
	}

	// 5. Android HTML response with valid token
	reqAndroid := httptest.NewRequest("GET", "/pair?token="+token, nil)
	reqAndroid.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36")
	recAndroid := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recAndroid, reqAndroid)

	if recAndroid.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", recAndroid.Code)
	}
	if !strings.Contains(recAndroid.Body.String(), "isbn_bridge_config.json") {
		t.Errorf("expected Android page to offer config file download")
	}
}

func TestPairHidesQRForMobile(t *testing.T) {
	srv, tokenMgr, forwarder := setupTestServer(t)
	token, err := tokenMgr.GetToken()
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}

	// iPhone UA (pairing-QR scan): carries token parameter, QR popup must be dismissed.
	reqMobile := httptest.NewRequest("GET", "/pair?token="+token, nil)
	reqMobile.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15")
	recMobile := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recMobile, reqMobile)
	if recMobile.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recMobile.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		forwarder.mu.Lock()
		n := forwarder.hideQRCount
		forwarder.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected HideQR after mobile /pair hit, got %d calls", n)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Desktop UA: popup stays up.
	reqDesktop := httptest.NewRequest("GET", "/pair?token="+token, nil)
	reqDesktop.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	recDesktop := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDesktop, reqDesktop)
	if recDesktop.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recDesktop.Code)
	}
	time.Sleep(100 * time.Millisecond)

	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	if forwarder.hideQRCount != 1 {
		t.Errorf("expected no extra HideQR for desktop UA, got %d", forwarder.hideQRCount)
	}
}
