package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	TokenManager *auth.TokenManager
	Verifier     *auth.Verifier
	Forwarder    Forwarder
	Port         int
	AppConfig    *config.Config
}

// Server implements the HTTP API for ISBN Bridge.
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
	return fmt.Sprintf("0.0.0.0:%d", s.port)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /isbn", s.handlePostISBN)
	s.mux.HandleFunc("GET /qr", s.handleGetQRHTML)
	s.mux.HandleFunc("GET /qr.png", s.handleGetQRPNG)
	s.mux.HandleFunc("POST /qr/show", s.handleShowQR)
	s.mux.HandleFunc("POST /token/refresh", s.handleRefreshToken)
	s.mux.HandleFunc("POST /token/ttl", s.handleSetTTL)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /pair", s.handlePair)
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
  <title>ISBN Bridge - Pairing</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #0f172a; color: #f8fafc; }
    .card { background: #1e293b; max-width: 360px; width: 90%%; padding: 28px 20px; border-radius: 20px; text-align: center; box-shadow: 0 10px 25px rgba(0,0,0,0.3); border: 1px solid #334155; }
    img { width: 240px; height: 240px; border-radius: 12px; margin: 16px auto; background: white; padding: 8px; display: block; }
    h2 { font-size: 1.15rem; font-weight: 600; margin: 0 0 6px 0; }
    p { font-size: 0.85rem; color: #94a3b8; margin: 0; }
  </style>
</head>
<body>
  <div class="card">
    <h2>ISBN Bridge</h2>
    <p>Inquadra con la fotocamera per abbinare</p>
    <img src="data:image/png;base64,%s" alt="QR Code" />
    <p style="font-size: 0.75rem; color: #64748b;">Token attivo: %s</p>
  </div>
</body>
</html>`, b64, token[:8]+"...")

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

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	activeToken, err := s.tokenMgr.GetToken()
	if err != nil {
		http.Error(w, "Failed to retrieve token: "+err.Error(), http.StatusInternalServerError)
		return
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

	ua := r.UserAgent()
	isIOS := strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "iPod")
	isAndroid := strings.Contains(ua, "Android")

	escapedJSON := url.QueryEscape(configJSON)
	primaryShortcutURI := fmt.Sprintf("shortcuts://run-shortcut?name=Pair%%20ISBN%%20Bridge&input=text&text=%s", escapedJSON)
	altShortcutURI := fmt.Sprintf("shortcuts://run-shortcut?name=ISBN%%20Bridge%%20Pair&input=text&text=%s", escapedJSON)

	badgeHTML := `<div class="badge"><span class="dot"></span> Server Connected</div>`
	if tokenMismatch {
		badgeHTML = `<div class="badge warn"><span class="dot"></span> Token Refreshed (Updated)</div>`
	}

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

		actionSection = fmt.Sprintf(`
    <a href=%q class="btn btn-primary">⚡ Tap to Pair iPhone</a>
    <a href=%q class="btn btn-secondary">Alternate: Run "ISBN Bridge Pair"</a>
    <p class="subtext">If Safari asks, tap <strong>Open in Shortcuts</strong> to finish pairing.</p>
`, primaryShortcutURI, altShortcutURI)
	} else if isAndroid {
		actionSection = fmt.Sprintf(`
    <button class="btn btn-primary" onclick="copyConfig()">📋 Copy Config JSON</button>
    <a href="data:application/json;charset=utf-8,%s" download="isbn_bridge_config.json" class="btn btn-secondary">💾 Download Config File</a>
    <p class="subtext">Use in Tasker, HTTP Shortcuts, or your scanner automation app.</p>
`, url.PathEscape(configJSON))
	} else {
		actionSection = fmt.Sprintf(`
    <a href=%q class="btn btn-primary">⚡ Launch iOS Shortcut</a>
    <button class="btn btn-secondary" onclick="copyConfig()">📋 Copy Config JSON</button>
`, primaryShortcutURI)
	}

	templateHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
  <title>Pair with ISBN Bridge</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; padding: 24px 16px; min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; }
    .card { background: #1e293b; width: 100%; max-width: 440px; border-radius: 20px; padding: 28px 24px; box-shadow: 0 10px 30px rgba(0,0,0,0.4); border: 1px solid #334155; text-align: center; }
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
    {{BADGE}}
    <h1>ISBN Bridge Pairing</h1>
    <p class="desc">Connecting your mobile barcode scanner to your PC desktop session.</p>

    {{ACTIONS}}

    <div class="info-box">
      <div class="info-row">
        <span class="info-label">Server Host:</span>
        <span class="info-val">{{SERVER_URL}}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Token TTL:</span>
        <span class="info-val">{{TTL_MINUTES}} min</span>
      </div>
    </div>
    <div id="toast">✅ Config copied to clipboard!</div>
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
