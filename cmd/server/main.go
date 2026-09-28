package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/denialbb/isbn-bridge/pkg/auth"
	"github.com/denialbb/isbn-bridge/pkg/config"
	"github.com/denialbb/isbn-bridge/pkg/netutil"
	"github.com/denialbb/isbn-bridge/pkg/server"
)

func main() {
	confFile := flag.String("conf", "scanner.conf", "Path to scanner.conf unified configuration file")
	portFlag := flag.Int("port", 0, "Override HTTP port for incoming iOS Shortcut requests")
	ahkPortFlag := flag.Int("ahk-port", 0, "Override desktop listener port")
	tokenFile := flag.String("token-file", "token.txt", "Path to file for persisting active token")
	hideFlag := flag.Bool("hide-console", false, "Hide server console window on Windows")
	netModeFlag := flag.String("net-mode", "", "Override network interface mode (auto, usb, lan)")
	serverIPFlag := flag.String("server-ip", "", "Override server IP for pairing QR and base URL")
	flag.Parse()

	initConsole()

	log.Println("==================================================")
	log.Println("             ISBN BRIDGE GO SERVER                ")
	log.Println("==================================================")

	// 1. Load or create scanner.conf
	appCfg, err := config.LoadOrCreate(*confFile)
	if err != nil {
		log.Printf("Warning: failed to load %s (%v), using default settings", *confFile, err)
		appCfg = config.Default()
	} else {
		log.Printf("Loaded configuration from %s", *confFile)
	}

	// CLI flags override config file if provided
	if *portFlag > 0 {
		appCfg.Port = *portFlag
	}
	if *ahkPortFlag > 0 {
		appCfg.AHKPort = *ahkPortFlag
	}
	if *hideFlag {
		appCfg.HideConsole = true
	}
	if *netModeFlag != "" {
		appCfg.NetworkMode = *netModeFlag
	}
	if *serverIPFlag != "" {
		appCfg.ServerIP = *serverIPFlag
	}

	tokenMgr := auth.NewTokenManager(*tokenFile, appCfg.TokenTTL)
	if _, err := tokenMgr.GetToken(); err != nil {
		log.Fatalf("Failed to initialize token: %v", err)
	}

	// Shared secret for the local desktop listener: proves that
	// /paste, /qr/show and /qr/hide requests come from this server.
	// Written next to the token file (0600); the client re-reads it
	// per request, so server restarts need no client restart.
	localSecret, err := auth.GenerateRandomToken(32)
	if err != nil {
		log.Fatalf("Failed to generate local secret: %v", err)
	}
	secretPath := filepath.Join(filepath.Dir(*tokenFile), "local_secret.txt")
	if err := os.WriteFile(secretPath, []byte(localSecret), 0600); err != nil {
		log.Printf("Warning: cannot persist local secret (%v); desktop paste auth disabled", err)
		localSecret = ""
	}

	shutdownChan := make(chan struct{})
	shutdownFunc := func() {
		select {
		case <-shutdownChan:
		default:
			close(shutdownChan)
		}
	}

	verifier := auth.NewVerifier(appCfg.MaxSkew)
	ahkTarget := fmt.Sprintf("http://127.0.0.1:%d", appCfg.AHKPort)
	forwarder := server.NewAHKForwarder(ahkTarget)
	forwarder.SetLocalSecret(localSecret)

	srv := server.NewServer(server.Config{
		TokenManager:     tokenMgr,
		Verifier:         verifier,
		Forwarder:        forwarder,
		Port:             appCfg.Port,
		AppConfig:        appCfg,
		ConsoleShow:      showConsole,
		ConsoleHide:      hideConsole,
		ConsoleToggle:    toggleConsole,
		ConsoleIsVisible: isConsoleVisible,
		ShutdownFunc:     shutdownFunc,
	})

	selectedIface, err := netutil.ResolveServerIP(appCfg.NetworkMode, appCfg.ServerIP)
	if err != nil {
		log.Printf("Warning: failed to resolve server IP (%v), using loopback fallback", err)
	}
	serverURL := netutil.FormatServerURL(selectedIface.IP, appCfg.Port)
	tokenMgr.SetBaseURL(serverURL)

	// Dynamically resolve server URL on demand for /qr, /pair, and /qr.png
	tokenMgr.SetURLResolver(func() string {
		iface, _ := netutil.ResolveServerIP(appCfg.NetworkMode, appCfg.ServerIP)
		return netutil.FormatServerURL(iface.IP, appCfg.Port)
	})

	log.Println("Active Token: initialized (scan QR code to pair)")
	log.Printf("Token TTL:    %v (auto-refreshes)", appCfg.TokenTTL)
	log.Printf("Timestamp skew window: ±%v", appCfg.MaxSkew)
	log.Printf("Rate limit:   %d req / %v per IP on POST /isbn", appCfg.RateLimitMax, appCfg.RateLimitWindow)
	log.Printf("Replay cache: %d signatures / %v TTL", appCfg.ReplaySize, appCfg.ReplayTTL)
	log.Printf("Network mode: %s (interface: %s, IP: %s, type: %s)",
		appCfg.NetworkMode, selectedIface.Name, selectedIface.IP, selectedIface.Type)
	log.Printf("Listening on: http://0.0.0.0:%d", appCfg.Port)
	log.Printf("Phone on Wi-Fi timing out? The port is likely firewalled: sudo ufw allow %d/tcp", appCfg.Port)
	log.Printf("  -> Mobile Pairing URL: %s/pair (access via QR scan)", serverURL)
	log.Printf("  -> Browser QR page:    %s/qr", serverURL)
	log.Printf("  -> ISBN Post URL:      %s/isbn", serverURL)
	log.Printf("Forwarding to desktop client at: %s", ahkTarget)
	log.Println("--------------------------------------------------")
	log.Println("Pairing QR is shown as a desktop popup and at /qr in a browser.")

	// Background interface monitor for dynamic USB hot-plug / unplug detection
	go func() {
		lastIP := selectedIface.IP
		lastType := selectedIface.Type
		ifaceTicker := time.NewTicker(2 * time.Second)
		defer ifaceTicker.Stop()

		for {
			select {
			case <-ifaceTicker.C:
				currentIface, err := netutil.ResolveServerIP(appCfg.NetworkMode, appCfg.ServerIP)
				if err != nil {
					continue
				}

				if currentIface.IP != lastIP {
					log.Println("--------------------------------------------------")
					log.Printf("Network interface changed: %s (%s) -> %s (%s)",
						lastIP, lastType, currentIface.IP, currentIface.Type)
					lastIP = currentIface.IP
					lastType = currentIface.Type
					newURL := netutil.FormatServerURL(currentIface.IP, appCfg.Port)
					tokenMgr.SetBaseURL(newURL)
					log.Printf("  -> Updated Mobile Pairing URL: %s/pair (access via QR scan)", newURL)
					log.Printf("  -> Updated Browser QR page:    %s/qr", newURL)
					log.Printf("  -> Updated ISBN Post URL:      %s/isbn", newURL)
					log.Println("--------------------------------------------------")

					if appCfg.QRAutoShowOnRefresh {
						_ = forwarder.ShowQR(context.Background())
					}
				}
			case <-shutdownChan:
				return
			}
		}
	}()

	// Trigger seamless centered QR popup in desktop client on startup
	if appCfg.QRAutoShowOnRefresh {
		go func() {
			for i := 0; i < 10; i++ {
				time.Sleep(500 * time.Millisecond)
				if err := forwarder.ShowQR(context.Background()); err == nil {
					log.Printf("Successfully requested QR modal popup from desktop client")
					return
				}
			}
			log.Printf("Note: desktop listener not reachable for startup QR popup")
		}()
	}

	// Background ticker to check and auto-refresh token
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if tokenMgr.IsExpired() {
				_, err := tokenMgr.RefreshToken()
				if err != nil {
					log.Printf("Error auto-refreshing expired token: %v", err)
					continue
				}
				log.Println("--------------------------------------------------")
				log.Println("Token expired! New token generated.")
				log.Println("Open /qr in a browser for the updated pairing code:")
				log.Println("--------------------------------------------------")

				if appCfg.QRAutoShowOnRefresh {
					_ = forwarder.ShowQR(context.Background())
				}
			}
		}
	}()

	httpServer := &http.Server{
		Addr:              srv.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			// A second instance would otherwise exit instantly with a
			// flashing window. Force the console visible, explain, and
			// pause so the message can be read.
			if isAddrInUse(err) {
				showConsole()
				fmt.Println()
				fmt.Println("Another ISBN Bridge server is already running on this port.")
				fmt.Println("Use the tray icon (ISBN-Bridge.exe) instead of starting a second server.")
				fmt.Println("Press Enter to exit.")
				fmt.Scanln()
			}
			os.Exit(1)
		}
	}()

	// Auto-hide console if configured
	if appCfg.HideConsole {
		hideConsole()
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case <-shutdownChan:
	}

	log.Println("Shutting down ISBN Bridge server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}
	log.Println("Server gracefully stopped.")
}

// isAddrInUse reports whether err is a TCP bind collision (a second
// server instance). Matches WSAEADDRINUSE on Windows and EADDRINUSE on
// Linux, with message fallbacks for wrapped errors.
func isAddrInUse(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == 10048 || errno == 98 {
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "Only one usage of each socket address")
}
