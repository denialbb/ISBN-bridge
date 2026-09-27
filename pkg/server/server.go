package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/denialbb/isbn-bridge/pkg/auth"
	"github.com/denialbb/isbn-bridge/pkg/config"
	"github.com/denialbb/isbn-bridge/pkg/isbn"
)

const maxBodyBytes = 16 * 1024 // 16 KB

// Config holds the dependencies for Server.
type Config struct {
	TokenManager     *auth.TokenManager
	Verifier         *auth.Verifier
	Forwarder        Forwarder
	Port             int
	AppConfig        *config.Config
	ConsoleShow      func()
	ConsoleHide      func()
	ConsoleToggle    func() bool
	ConsoleIsVisible func() bool
	ShutdownFunc     func()
}

// Server implements the HTTP API for ISBN Bridge.
type Server struct {
	tokenMgr         *auth.TokenManager
	verifier         *auth.Verifier
	forwarder        Forwarder
	appConfig        *config.Config
	port             int
	mux              *http.ServeMux
	langs            *pageLangs
	replay           *replayCache
	limiter          *rateLimiter
	consoleShow      func()
	consoleHide      func()
	consoleToggle    func() bool
	consoleIsVisible func() bool
	shutdownFunc     func()
}

// NewServer initializes a new Server.
func NewServer(cfg Config) *Server {
	if cfg.Port <= 0 {
		cfg.Port = 8765
	}

	s := &Server{
		tokenMgr:         cfg.TokenManager,
		verifier:         cfg.Verifier,
		forwarder:        cfg.Forwarder,
		appConfig:        cfg.AppConfig,
		port:             cfg.Port,
		mux:              http.NewServeMux(),
		langs:            loadPageLangs(),
		replay:           newReplayCache(replaySize(cfg.AppConfig), replayTTL(cfg.AppConfig)),
		limiter:          newRateLimiter(rateLimitMax(cfg.AppConfig), rateLimitWindow(cfg.AppConfig)),
		consoleShow:      cfg.ConsoleShow,
		consoleHide:      cfg.ConsoleHide,
		consoleToggle:    cfg.ConsoleToggle,
		consoleIsVisible: cfg.ConsoleIsVisible,
		shutdownFunc:     cfg.ShutdownFunc,
	}

	s.routes()
	return s
}

// The LAN-hardening tunables below default to the report values when no
// AppConfig is attached (e.g. in tests): 2 req/10s per IP, 100 signatures
// remembered for 60s.

func replaySize(cfg *config.Config) int {
	if cfg == nil {
		return 100
	}
	return cfg.ReplaySize
}

func replayTTL(cfg *config.Config) time.Duration {
	if cfg == nil {
		return 60 * time.Second
	}
	return cfg.ReplayTTL
}

func rateLimitMax(cfg *config.Config) int {
	if cfg == nil {
		return 2
	}
	return cfg.RateLimitMax
}

func rateLimitWindow(cfg *config.Config) time.Duration {
	if cfg == nil {
		return 10 * time.Second
	}
	return cfg.RateLimitWindow
}

// Handler returns the underlying http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Addr returns the server listen address.
func (s *Server) Addr() string {
	return fmt.Sprintf("0.0.0.0:%d", s.port)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /isbn", s.handlePostISBN)
	s.mux.HandleFunc("GET /qr", s.handleGetQRHTML)
	s.mux.HandleFunc("GET /qr.png", s.handleGetQRPNG)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /pair", s.handlePair)
	s.mux.HandleFunc("GET /", s.handleRoot)
	// Control plane: desktop client only. The phone never calls these;
	// gating them to loopback keeps LAN neighbors from rotating the
	// token, toggling the console, or stopping the server.
	s.mux.HandleFunc("POST /qr/show", requireLoopback(s.handleShowQR))
	s.mux.HandleFunc("POST /token/refresh", requireLoopback(s.handleRefreshToken))
	s.mux.HandleFunc("POST /token/ttl", requireLoopback(s.handleSetTTL))
	s.mux.HandleFunc("POST /console/show", requireLoopback(s.handleConsoleShow))
	s.mux.HandleFunc("POST /console/hide", requireLoopback(s.handleConsoleHide))
	s.mux.HandleFunc("POST /console/toggle", requireLoopback(s.handleConsoleToggle))
	s.mux.HandleFunc("GET /console", requireLoopback(s.handleConsoleStatus))
	s.mux.HandleFunc("POST /shutdown", requireLoopback(s.handleShutdown))
}

// requireLoopback rejects requests that did not arrive over the local
// machine (127.0.0.1 or ::1). Used for the control endpoints the
// desktop client calls; the phone only needs /isbn, /qr*, /pair and
// /health.
func requireLoopback(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := clientIP(r)
		if host != "127.0.0.1" && host != "::1" {
			log.Printf("Control endpoint %s rejected for non-loopback client %s", r.URL.Path, r.RemoteAddr)
			http.Error(w, "Forbidden: localhost only", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (s *Server) handlePostISBN(w http.ResponseWriter, r *http.Request) {
	// 0. Per-IP rate limit (LAN anti-spam) before any expensive work.
	if s.limiter != nil && !s.limiter.allow(clientIP(r)) {
		log.Printf("Rate limit exceeded (Client IP: %s)", r.RemoteAddr)
		http.Error(w, "Too many requests, slow down", http.StatusTooManyRequests)
		return
	}

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

	// 3b. Single-use signatures: reject replays of an accepted request.
	if s.replay != nil {
		if s.replay.checkAndMark(normalizeSignature(authHeader), time.Now()) {
			log.Printf("Replay rejected: signature already used (Client IP: %s)", r.RemoteAddr)
			http.Error(w, "Conflict: signature already used", http.StatusConflict)
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

// isMobileUserAgent reports whether ua looks like a phone/tablet browser
// (used to detect a pairing-QR scan without touching the iOS Shortcuts).
func isMobileUserAgent(ua string) bool {
	for _, token := range []string{"iPhone", "iPad", "iPod", "Android", "Mobile"} {
		if strings.Contains(ua, token) {
			return true
		}
	}
	return false
}

// clientIP returns the host portion of r.RemoteAddr without the port,
// used as the rate-limit bucket key.
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// normalizeSignature canonicalizes an Authorization header value for
// replay comparison (case-insensitive hex, optional Bearer prefix).
func normalizeSignature(authHeader string) string {
	trimmed := strings.TrimSpace(authHeader)
	if len(trimmed) > 7 && strings.EqualFold(trimmed[:7], "bearer ") {
		trimmed = strings.TrimSpace(trimmed[7:])
	}
	return strings.ToLower(trimmed)
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
	lang := s.detectLanguage(r)

	prefix := token
	if len(token) > 8 {
		prefix = token[:8]
	}
	subtitle := s.langs.get(lang, "qr_subtitle")
	tokenNote := strings.ReplaceAll(s.langs.get(lang, "qr_token_note"), "{prefix}", prefix)
	qrTitle := s.langs.get(lang, "qr_title")

	var bar strings.Builder
	bar.WriteString(`<div class="lang-bar">`)
	for _, code := range s.langs.codes() {
		cls := "lang-btn"
		if code == lang {
			cls += " active"
		}
		fmt.Fprintf(&bar, `<a href="?lang=%s" class="%s">%s</a>`, code, cls, s.langs.name(code))
	}
	bar.WriteString(`</div>`)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #0f172a; color: #f8fafc; }
    .card { background: #1e293b; max-width: 360px; width: 90%%; padding: 24px 20px; border-radius: 20px; text-align: center; box-shadow: 0 10px 25px rgba(0,0,0,0.3); border: 1px solid #334155; position: relative; }
    .lang-bar { display: flex; justify-content: flex-end; gap: 6px; margin-bottom: 8px; }
    .lang-btn { font-size: 11px; padding: 3px 8px; border-radius: 6px; text-decoration: none; color: #94a3b8; background: #0f172a; border: 1px solid #334155; font-weight: 600; }
    .lang-btn.active { color: #ffffff; background: #2563eb; border-color: #3b82f6; }
    img { width: 240px; height: 240px; border-radius: 12px; margin: 14px auto; background: white; padding: 8px; display: block; }
    h2 { font-size: 1.15rem; font-weight: 600; margin: 0 0 6px 0; }
    p { font-size: 0.85rem; color: #94a3b8; margin: 0; }
  </style>
</head>
<body>
  <div class="card">
    %s
    <h2>ISBN Bridge</h2>
    <p>%s</p>
    <img src="data:image/png;base64,%s" alt="QR Code" />
    <p style="font-size: 0.75rem; color: #64748b;">%s</p>
  </div>
</body>
</html>`, lang, qrTitle, bar.String(), subtitle, b64, tokenNote)

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

	log.Printf("Token refreshed (prefix %.8s...)", newToken)

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

	log.Printf("Token TTL updated to %d minutes (prefix %.8s...)", mins, newToken)

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

func (s *Server) handleConsoleShow(w http.ResponseWriter, r *http.Request) {
	if s.consoleShow != nil {
		s.consoleShow()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "visible": true})
}

func (s *Server) handleConsoleHide(w http.ResponseWriter, r *http.Request) {
	if s.consoleHide != nil {
		s.consoleHide()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "visible": false})
}

func (s *Server) handleConsoleToggle(w http.ResponseWriter, r *http.Request) {
	visible := false
	if s.consoleToggle != nil {
		visible = s.consoleToggle()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "visible": visible})
}

func (s *Server) handleConsoleStatus(w http.ResponseWriter, r *http.Request) {
	visible := false
	if s.consoleIsVisible != nil {
		visible = s.consoleIsVisible()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "visible": visible})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})
	if s.shutdownFunc != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.shutdownFunc()
		}()
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/qr", http.StatusTemporaryRedirect)
}

func (s *Server) detectLanguage(r *http.Request) string {
	if s.langs == nil {
		s.langs = loadPageLangs()
	}
	// Explicit ?lang= wins, then scanner.conf, then browser preference.
	if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang"))); s.langs.has(q) {
		return q
	}
	if s.appConfig != nil {
		if l := strings.ToLower(strings.TrimSpace(s.appConfig.Language)); s.langs.has(l) {
			return l
		}
	}
	accept := strings.ToLower(r.Header.Get("Accept-Language"))
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if i := strings.Index(part, ";"); i >= 0 {
			part = strings.TrimSpace(part[:i])
		}
		if i := strings.Index(part, "-"); i >= 0 {
			part = part[:i]
		}
		if part != "" && s.langs.has(part) {
			return part
		}
	}
	return "en"
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	activeToken, err := s.tokenMgr.GetToken()
	if err != nil {
		http.Error(w, "Failed to retrieve token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// A phone opening the pairing link means the QR was just scanned:
	// dismiss the desktop popup. Desktop UAs leave it up.
	if isMobileUserAgent(r.UserAgent()) && s.forwarder != nil {
		go func() {
			_ = s.forwarder.HideQR(context.Background())
		}()
	}

	tokenParam := strings.TrimSpace(r.URL.Query().Get("token"))
	tokenMismatch := tokenParam != "" && tokenParam != activeToken

	serverURL := s.tokenMgr.BaseURL()
	if serverURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		serverURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	configMap := map[string]string{
		"url":   serverURL,
		"token": activeToken,
	}
	configBytes, _ := json.Marshal(configMap)
	configJSON := string(configBytes)

	if r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":         "ok",
			"url":            serverURL,
			"token":          activeToken,
			"token_mismatch": tokenMismatch,
			"ttl_minutes":    int(s.tokenMgr.TTL().Minutes()),
		})
		return
	}

	lang := s.detectLanguage(r)
	T := func(key string) string { return s.langs.get(lang, key) }

	ua := r.UserAgent()
	isIOS := strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "iPod")
	isAndroid := strings.Contains(ua, "Android")

	escapedJSON := url.QueryEscape(configJSON)
	primaryShortcutURI := fmt.Sprintf("shortcuts://run-shortcut?name=Pair%%20ISBN%%20Bridge&input=text&text=%s", escapedJSON)
	altShortcutURI := fmt.Sprintf("shortcuts://run-shortcut?name=ISBN%%20Bridge%%20Pair&input=text&text=%s", escapedJSON)

	badgeHTML := `<div class="badge"><span class="dot"></span> ` + T("pair_badge_connected") + `</div>`
	if tokenMismatch {
		badgeHTML = `<div class="badge warn"><span class="dot"></span> ` + T("pair_badge_refreshed") + `</div>`
	}

	title := T("pair_title")
	desc := T("pair_desc")
	serverLabel := T("pair_server_label")
	ttlLabel := T("pair_ttl_label")
	toastMsg := T("pair_toast")

	var bar strings.Builder
	bar.WriteString(`<div class="lang-bar">`)
	for _, code := range s.langs.codes() {
		cls := "lang-btn"
		if code == lang {
			cls += " active"
		}
		fmt.Fprintf(&bar, `<a href="?lang=%s&token=%s" class="%s">%s</a>`,
			code, url.QueryEscape(tokenParam), cls, s.langs.name(code))
	}
	bar.WriteString(`</div>`)
	langBar := bar.String()

	autoRedirectScript := ""
	actionSection := ""

	if isIOS {
		autoRedirectScript = fmt.Sprintf(`
  <script>
    window.onload = function() {
      setTimeout(function() {
        window.location.href = %q;
      }, 400);
    };
  </script>`, primaryShortcutURI)

		iosPrimary := T("pair_ios_primary")
		iosAlt := T("pair_ios_alt")
		iosSub := T("pair_ios_sub")

		actionSection = fmt.Sprintf(`
    <a href=%q class="btn btn-primary">%s</a>
    <a href=%q class="btn btn-secondary">%s</a>
    <p class="subtext">%s</p>
`, primaryShortcutURI, iosPrimary, altShortcutURI, iosAlt, iosSub)
	} else if isAndroid {
		andCopy := T("pair_and_copy")
		andDownload := T("pair_and_download")
		andSub := T("pair_and_sub")

		actionSection = fmt.Sprintf(`
    <button class="btn btn-primary" onclick="copyConfig()">%s</button>
    <a href="data:application/json;charset=utf-8,%s" download="isbn_bridge_config.json" class="btn btn-secondary">%s</a>
    <p class="subtext">%s</p>
`, andCopy, url.PathEscape(configJSON), andDownload, andSub)
	} else {
		launchText := T("pair_launch")
		copyText := T("pair_copy")
		actionSection = fmt.Sprintf(`
    <a href=%q class="btn btn-primary">%s</a>
    <button class="btn btn-secondary" onclick="copyConfig()">%s</button>
`, primaryShortcutURI, launchText, copyText)
	}

	templateHTML := `<!DOCTYPE html>
<html lang="` + lang + `">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
  <title>` + title + `</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; padding: 24px 16px; min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; }
    .card { background: #1e293b; width: 100%; max-width: 440px; border-radius: 20px; padding: 24px 20px; box-shadow: 0 10px 30px rgba(0,0,0,0.4); border: 1px solid #334155; text-align: center; }
    .lang-bar { display: flex; justify-content: flex-end; gap: 6px; margin-bottom: 12px; }
    .lang-btn { font-size: 11px; padding: 4px 9px; border-radius: 6px; text-decoration: none; color: #94a3b8; background: #0f172a; border: 1px solid #334155; font-weight: 600; }
    .lang-btn.active { color: #ffffff; background: #2563eb; border-color: #3b82f6; }
    .badge { display: inline-flex; align-items: center; gap: 6px; padding: 6px 14px; border-radius: 999px; font-size: 13px; font-weight: 600; margin-bottom: 16px; background: rgba(34, 197, 94, 0.15); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
    .badge.warn { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border-color: rgba(245, 158, 11, 0.3); }
    .dot { width: 8px; height: 8px; border-radius: 50%; background: currentColor; }
    h1 { font-size: 22px; font-weight: 700; margin-bottom: 8px; color: #ffffff; }
    p.desc { font-size: 14px; color: #94a3b8; margin-bottom: 24px; line-height: 1.5; }
    .btn { display: block; width: 100%; padding: 14px 18px; border-radius: 12px; font-size: 16px; font-weight: 600; text-decoration: none; text-align: center; cursor: pointer; transition: all 0.2s ease; border: none; margin-bottom: 12px; }
    .btn-primary { background: #2563eb; color: #ffffff; }
    .btn-primary:hover { background: #1d4ed8; }
    .btn-secondary { background: #334155; color: #e2e8f0; font-size: 14px; }
    .btn-secondary:hover { background: #475569; }
    .subtext { font-size: 13px; color: #64748b; margin-top: 8px; margin-bottom: 16px; line-height: 1.4; }
    .info-box { background: #0f172a; border-radius: 12px; padding: 14px; margin-top: 16px; border: 1px solid #334155; text-align: left; font-size: 13px; }
    .info-row { display: flex; justify-content: space-between; margin-bottom: 8px; }
    .info-row:last-child { margin-bottom: 0; }
    .info-label { color: #64748b; font-weight: 500; }
    .info-val { color: #cbd5e1; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; word-break: break-all; }
    #toast { display: none; margin-top: 12px; font-size: 13px; color: #4ade80; }
  </style>
  {{AUTO_REDIRECT}}
</head>
<body>
  <div class="card">
    {{LANG_BAR}}
    {{BADGE}}
    <h1>` + title + `</h1>
    <p class="desc">` + desc + `</p>

    {{ACTIONS}}

    <div class="info-box">
      <div class="info-row">
        <span class="info-label">` + serverLabel + `</span>
        <span class="info-val">{{SERVER_URL}}</span>
      </div>
      <div class="info-row">
        <span class="info-label">` + ttlLabel + `</span>
        <span class="info-val">{{TTL_MINUTES}} min</span>
      </div>
    </div>
    <div id="toast">` + toastMsg + `</div>
  </div>

  <script>
    function copyConfig() {
      const cfg = {{CONFIG_JSON_LITERAL}};
      navigator.clipboard.writeText(cfg).then(function() {
        const t = document.getElementById("toast");
        t.style.display = "block";
        setTimeout(() => { t.style.display = "none"; }, 3000);
      });
    }
  </script>
</body>
</html>`

	replacer := strings.NewReplacer(
		"{{AUTO_REDIRECT}}", autoRedirectScript,
		"{{LANG_BAR}}", langBar,
		"{{BADGE}}", badgeHTML,
		"{{ACTIONS}}", actionSection,
		"{{SERVER_URL}}", serverURL,
		"{{TTL_MINUTES}}", strconv.Itoa(int(s.tokenMgr.TTL().Minutes())),
		"{{CONFIG_JSON_LITERAL}}", strconv.Quote(configJSON),
	)
	html := replacer.Replace(templateHTML)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
