package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// Forwarder defines the contract for sending validated ISBNs to an external recipient (e.g. AutoHotkey).
type Forwarder interface {
	Forward(ctx context.Context, isbn string) error
}

// AHKForwarder sends validated ISBNs via HTTP POST to the local AutoHotkey script.
type AHKForwarder struct {
	url    string
	client *http.Client
}

// NewAHKForwarder creates a new AHKForwarder targeting the given URL.
func NewAHKForwarder(targetURL string) *AHKForwarder {
	if targetURL == "" {
		targetURL = "http://127.0.0.1:8766/paste"
	}
	return &AHKForwarder{
		url: targetURL,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// Forward delivers the ISBN to the local AutoHotkey listener.
func (f *AHKForwarder) Forward(ctx context.Context, isbn string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewBufferString(isbn))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to forward ISBN to AutoHotkey at %s: %w", f.url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AutoHotkey returned status %d", resp.StatusCode)
	}
	return nil
}
