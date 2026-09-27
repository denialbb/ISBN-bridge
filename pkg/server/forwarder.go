package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Forwarder defines the contract for communicating with AutoHotkey.
type Forwarder interface {
	Forward(ctx context.Context, isbn string) error
	ShowQR(ctx context.Context) error
	HideQR(ctx context.Context) error
}

// LocalSecretHeader authenticates the Go forwarder to the local
// AutoHotkey listener. The secret is generated at server startup and
// written to local_secret.txt next to the token file (0600).
const LocalSecretHeader = "X-ISBN-Bridge-Local"

// AHKForwarder sends actions to the local AutoHotkey script via HTTP.
type AHKForwarder struct {
	baseURL     string
	client      *http.Client
	localSecret string
}

// NewAHKForwarder creates a new AHKForwarder targeting the given base URL or port.
func NewAHKForwarder(target string) *AHKForwarder {
	if target == "" {
		target = "http://127.0.0.1:8766"
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://127.0.0.1:" + target
	}
	target = strings.TrimSuffix(target, "/")
	target = strings.TrimSuffix(target, "/paste")

	return &AHKForwarder{
		baseURL: target,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// SetLocalSecret sets the shared secret sent to the AutoHotkey
// listener. Empty means no secret header is sent.
func (f *AHKForwarder) SetLocalSecret(secret string) {
	f.localSecret = secret
}

// Forward delivers the validated ISBN to the local AutoHotkey listener.
func (f *AHKForwarder) Forward(ctx context.Context, isbn string) error {
	return f.post(ctx, "/paste", isbn)
}

// ShowQR instructs AutoHotkey to pop up the seamless centered QR code.
func (f *AHKForwarder) ShowQR(ctx context.Context) error {
	return f.post(ctx, "/qr/show", "")
}

// HideQR instructs AutoHotkey to dismiss the centered QR code (e.g. after successful scan).
func (f *AHKForwarder) HideQR(ctx context.Context) error {
	return f.post(ctx, "/qr/hide", "")
}

func (f *AHKForwarder) post(ctx context.Context, path, body string) error {
	url := f.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if f.localSecret != "" {
		req.Header.Set(LocalSecretHeader, f.localSecret)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact AutoHotkey at %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AutoHotkey returned status %d for %s", resp.StatusCode, path)
	}
	return nil
}
