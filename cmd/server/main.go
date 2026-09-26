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

	"github.com/denialbb/biblios-scanner/pkg/auth"
	"github.com/denialbb/biblios-scanner/pkg/server"
)

func main() {
	port := flag.Int("port", 8765, "HTTP port for incoming iOS Shortcut requests")
	ahkURL := flag.String("ahk", "http://127.0.0.1:8766/paste", "Target URL of local AutoHotkey paste listener")
	tokenFile := flag.String("token-file", "token.txt", "Path to file for persisting active token")
	ttl := flag.Duration("ttl", time.Hour, "Token time-to-live before automatic rotation")
	maxSkew := flag.Duration("max-skew", 15*time.Minute, "Maximum allowed clock skew for request timestamps")
	noQR := flag.Bool("no-terminal-qr", false, "Disable printing QR code in terminal")
	flag.Parse()

	log.Println("==================================================")
	log.Println("           BIBLIOS SCANNER GO SERVER              ")
	log.Println("==================================================")

	tokenMgr := auth.NewTokenManager(*tokenFile, *ttl)
	token, err := tokenMgr.GetToken()
	if err != nil {
		log.Fatalf("Failed to initialize token: %v", err)
	}

	verifier := auth.NewVerifier(*maxSkew)
	forwarder := server.NewAHKForwarder(*ahkURL)

	srv := server.NewServer(server.Config{
		TokenManager: tokenMgr,
		Verifier:     verifier,
		Forwarder:    forwarder,
		Port:         *port,
	})

	localIP := getOutboundIP()
	log.Printf("Active Token: %s", token)
	log.Printf("Token file:   %s", *tokenFile)
	log.Printf("Token TTL:    %v (auto-refreshes)", *ttl)
	log.Printf("Listening on: http://0.0.0.0:%d", *port)
	if localIP != "" {
		log.Printf("  -> iOS Shortcut URL: http://%s:%d/isbn", localIP, *port)
		log.Printf("  -> Browser QR page:   http://%s:%d/qr", localIP, *port)
	}
	log.Printf("Forwarding to AutoHotkey at: %s", *ahkURL)
	log.Println("--------------------------------------------------")

	if !*noQR {
		fmt.Println("\nScan this QR code with your iPhone to import the active token:")
		if err := tokenMgr.PrintTerminalQR(os.Stdout); err != nil {
			log.Printf("Failed to render terminal QR code: %v", err)
		}
		fmt.Println()
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
					tokenMgr.PrintTerminalQR(os.Stdout)
				}
				log.Println("--------------------------------------------------")
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

	log.Println("Shutting down Biblios server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}
	log.Println("Server gracefully stopped.")
}

func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
