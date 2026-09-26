package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/denialbb/biblios-scanner/pkg/auth"
	"github.com/denialbb/biblios-scanner/pkg/config"
	"github.com/denialbb/biblios-scanner/pkg/isbn"
)

const maxBodyBytes = 16 * 1024 // 16 KB

// Config holds the dependencies for Server.
type Config struct {
	TokenManager *auth.TokenManager
	Verifier     *auth.Verifier
	Forwarder    Forwarder
	Port         int
	AppConfig    *config.Config
}

// Server implements the HTTP API for Biblios Scanner.
type Server struct {
	tokenMgr  *auth.TokenManager
	verifier  *auth.Verifier
	forwarder Forwarder
	appConfig *config.Config
	port      int
	mux       *http.ServeMux
}

// NewServer initializes a new Server.
func NewServer(cfg Config) *Server {
	if cfg.Port <= 0 {
		cfg.Port = 8765
	}

	s := &Server{
		tokenMgr:  cfg.TokenManager,
		verifier:  cfg.Verifier,
		forwarder: cfg.Forwarder,
		appConfig: cfg.AppConfig,
		port:      cfg.Port,
		mux:       http.NewServeMux(),
	}

	s.routes()
	return s
}

// Handler returns the underlying http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Addr returns the server listen address.
func (s *Server) Addr() string {
	return fmt.Sprintf(":%d", s.port)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /isbn", s.handlePostISBN)
	s.mux.HandleFunc("GET /qr", s.handleGetQRHTML)
	s.mux.HandleFunc("GET /qr.png", s.handleGetQRPNG)
	s.mux.HandleFunc("POST /qr/show", s.handleShowQR)
	s.mux.HandleFunc("POST /token/refresh", s.handleRefreshToken)
	s.mux.HandleFunc("POST /token/ttl", s.handleSetTTL)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /", s.handleRoot)
}

func (s *Server) handlePostISBN(w http.ResponseWriter, r *http.Request) {
	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		http.Error(w, "Payload too large or read error", http.StatusBadRequest)
		return
	}

	rawISBN := strings.TrimSpace(string(bodyBytes))
	if rawISBN == "" {
		http.Error(w, "Missing ISBN in body", http.StatusBadRequest)
		return
	}

	// 1. Validate ISBN format & checksum
	normalizedISBN, err := isbn.Validate(rawISBN)
	if err != nil {
		log.Printf("Invalid ISBN rejected: %q (%v)", rawISBN, err)
		http.Error(w, "Invalid ISBN: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 2. Fetch active token
	token, err := s.tokenMgr.GetToken()
	if err != nil {
		log.Printf("Token manager error: %v", err)
		http.Error(w, "Internal token error", http.StatusInternalServerError)
		return
	}

	// 3. Verify SHA-256 signature
	authHeader := r.Header.Get("Authorization")
	timestamp := r.Header.Get("Timestamp")

	// Verify using raw string that client hashed: rawISBN|timestamp|token
	if err := s.verifier.Verify(rawISBN, timestamp, authHeader, token); err != nil {
		// Fallback: also check if client hashed the normalized ISBN
		if errNorm := s.verifier.Verify(normalizedISBN, timestamp, authHeader, token); errNorm != nil {
			log.Printf("Auth verification failed: %v (Client IP: %s, Timestamp: %q)", err, r.RemoteAddr, timestamp)
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
	}

	log.Printf("Authenticated valid ISBN: %s (Client: %s)", normalizedISBN, r.RemoteAddr)

	// 4. Forward to AutoHotkey
	if s.forwarder != nil {
		if err := s.forwarder.Forward(r.Context(), normalizedISBN); err != nil {
			log.Printf("Forwarding to AutoHotkey failed: %v", err)
			http.Error(w, "Desktop client error: "+err.Error(), http.StatusBadGateway)
			return
		}
		go func() {
			_ = s.forwarder.HideQR(context.Background())
		}()
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleGetQRPNG(w http.ResponseWriter, r *http.Request) {
	png, err := s.tokenMgr.GenerateQRCodePNG()
	if err != nil {
		http.Error(w, "Failed to generate QR code: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write(png)
}

func (s *Server) handleShowQR(w http.ResponseWriter, r *http.Request) {
	if s.forwarder != nil {
		if err := s.forwarder.ShowQR(r.Context()); err != nil {
			log.Printf("Notice: AutoHotkey ShowQR call failed: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "shown"})
}

func (s *Server) handleGetQRHTML(w http.ResponseWriter, r *http.Request) {
	png, err := s.tokenMgr.GenerateQRCodePNG()
	if err != nil {
		http.Error(w, "Failed to generate QR code: "+err.Error(), http.StatusInternalServerError)
		return
	}

	b64 := base64.StdEncoding.EncodeToString(png)
	token, _ := s.tokenMgr.GetToken()

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Biblios Token QR</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; text-align: center; padding: 40px 20px; background: #f8fafc; color: #1e293b; }
    .card { background: white; max-width: 440px; margin: 0 auto; padding: 30px; border-radius: 16px; box-shadow: 0 4px 20px rgba(0,0,0,0.08); }
    img { width: 260px; height: 260px; border-radius: 8px; margin: 15px 0; border: 1px solid #e2e8f0; }
    .token { font-family: monospace; font-size: 13px; background: #f1f5f9; padding: 10px; border-radius: 6px; word-break: break-all; margin: 15px 0; border: 1px solid #cbd5e1; }
    button { background: #2563eb; color: white; border: none; padding: 10px 20px; font-size: 15px; border-radius: 8px; cursor: pointer; font-weight: 500; }
    button:hover { background: #1d4ed8; }
    .note { font-size: 12px; color: #64748b; margin-top: 15px; }
  </style>
</head>
<body>
  <div class="card">
    <h2>Scan Token for Shortcuts</h2>
    <p>Scan this QR code with your iPhone token updater shortcut.</p>
    <img src="data:image/png;base64,%s" alt="Token QR Code" />
    <div class="token">%s</div>
    <form method="POST" action="/token/refresh">
      <button type="submit">Rotate / Regenerate Token</button>
    </form>
    <div class="note">Tokens automatically refresh every hour for security.</div>
  </div>
</body>
</html>`, b64, token)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	newToken, err := s.tokenMgr.RefreshToken()
	if err != nil {
		http.Error(w, "Failed to refresh token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Token refreshed: %s", newToken)

	if s.forwarder != nil {
		go func() {
			_ = s.forwarder.ShowQR(context.Background())
		}()
	}

	// If form submission from browser, redirect back to /qr
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/qr", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "refreshed",
		"token":  newToken,
	})
}

func (s *Server) handleSetTTL(w http.ResponseWriter, r *http.Request) {
	minutesStr := r.URL.Query().Get("minutes")
	if minutesStr == "" {
		minutesStr = r.FormValue("minutes")
	}

	mins, err := strconv.Atoi(minutesStr)
	if err != nil || mins <= 0 {
		http.Error(w, "Invalid or missing minutes parameter", http.StatusBadRequest)
		return
	}

	ttl := time.Duration(mins) * time.Minute
	s.tokenMgr.SetTTL(ttl)

	if s.appConfig != nil {
		_ = s.appConfig.SetTokenTTL(ttl)
	}

	newToken, err := s.tokenMgr.RefreshToken()
	if err != nil {
		http.Error(w, "Failed to refresh token with new TTL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Token TTL updated to %d minutes. New token: %s", mins, newToken)

	if s.forwarder != nil {
		go func() {
			_ = s.forwarder.ShowQR(context.Background())
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"ttl_minutes": mins,
		"token":       newToken,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"expired":     s.tokenMgr.IsExpired(),
		"ttl_minutes": int(s.tokenMgr.TTL().Minutes()),
		"server_time": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/qr", http.StatusTemporaryRedirect)
}
