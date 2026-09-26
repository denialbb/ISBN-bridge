package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/denialbb/isbn-bridge/pkg/auth"
	"github.com/denialbb/isbn-bridge/pkg/config"
	"github.com/denialbb/isbn-bridge/pkg/server"
)

func main() {
	confFile := flag.String("conf", "scanner.conf", "Path to scanner.conf unified configuration file")
	portFlag := flag.Int("port", 0, "Override HTTP port for incoming iOS Shortcut requests")
	ahkPortFlag := flag.Int("ahk-port", 0, "Override AutoHotkey listener port")
	tokenFile := flag.String("token-file", "token.txt", "Path to file for persisting active token")
	noQR := flag.Bool("no-terminal-qr", false, "Disable printing ANSI QR code to terminal")
	flag.Parse()

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

	tokenMgr := auth.NewTokenManager(*tokenFile, appCfg.TokenTTL)
	token, err := tokenMgr.GetToken()
	if err != nil {
		log.Fatalf("Failed to initialize token: %v", err)
	}

	verifier := auth.NewVerifier(appCfg.MaxSkew)
	ahkTarget := fmt.Sprintf("http://127.0.0.1:%d", appCfg.AHKPort)
	forwarder := server.NewAHKForwarder(ahkTarget)

	srv := server.NewServer(server.Config{
		TokenManager: tokenMgr,
		Verifier:     verifier,
		Forwarder:    forwarder,
		Port:         appCfg.Port,
		AppConfig:    appCfg,
	})

	localIP := getOutboundIP()
	if localIP == "" {
		localIP = "127.0.0.1"
	}
	serverURL := fmt.Sprintf("http://%s:%d", localIP, appCfg.Port)
	tokenMgr.SetBaseURL(serverURL)

	log.Printf("Active Token: %s", token)
	log.Printf("Token TTL:    %v (auto-refreshes)", appCfg.TokenTTL)
	log.Printf("Listening on: http://0.0.0.0:%d", appCfg.Port)
	log.Printf("  -> Mobile Pairing URL: %s/pair?token=%s", serverURL, token)
	log.Printf("  -> Browser QR page:    %s/qr", serverURL)
	log.Printf("  -> ISBN Post URL:      %s/isbn", serverURL)
	log.Printf("Forwarding to AutoHotkey at: %s", ahkTarget)
	log.Println("--------------------------------------------------")

	if !*noQR {
		fmt.Println("\nScan this QR code with your phone camera to pair automatically:")
		if err := tokenMgr.PrintTerminalQR(os.Stdout); err != nil {
			log.Printf("Failed to render terminal QR code: %v", err)
		}
		fmt.Println()
	}

	// Trigger seamless centered QR popup in AutoHotkey on startup
	if appCfg.QRAutoShowOnRefresh {
		go func() {
			for i := 0; i < 10; i++ {
				time.Sleep(500 * time.Millisecond)
				if err := forwarder.ShowQR(context.Background()); err == nil {
					log.Printf("Successfully requested QR modal popup from AutoHotkey")
					return
				}
			}
			log.Printf("Note: AutoHotkey listener not reachable for startup QR popup")
		}()
	}

	// Background ticker to check and auto-refresh token
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if tokenMgr.IsExpired() {
				newToken, err := tokenMgr.RefreshToken()
				if err != nil {
					log.Printf("Error auto-refreshing expired token: %v", err)
					continue
				}
				log.Println("--------------------------------------------------")
				log.Printf("🔄 Token expired! New token generated: %s", newToken)
				log.Println("Scan updated QR code below or open /qr in browser:")
				if !*noQR {
					_ = tokenMgr.PrintTerminalQR(os.Stdout)
				}
				log.Println("--------------------------------------------------")

				if appCfg.QRAutoShowOnRefresh {
					_ = forwarder.ShowQR(context.Background())
				}
			}
		}
	}()

	httpServer := &http.Server{
		Addr:    srv.Addr(),
		Handler: srv.Handler(),
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down ISBN Bridge server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}
	log.Println("Server gracefully stopped.")
}

func getOutboundIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 500*time.Millisecond)
	if err == nil {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP.String()
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return ""
}
