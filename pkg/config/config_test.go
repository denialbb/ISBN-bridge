package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrDefault(t *testing.T) {
	tempDir := t.TempDir()
	confPath := filepath.Join(tempDir, "scanner.conf")

	// 1. Loading non-existent file should return defaults and create the file
	cfg, err := LoadOrCreate(confPath)
	if err != nil {
		t.Fatalf("unexpected error creating default config: %v", err)
	}

	if cfg.Port != 8765 {
		t.Errorf("expected default Port 8765, got %d", cfg.Port)
	}
	if cfg.AHKPort != 8766 {
		t.Errorf("expected default AHKPort 8766, got %d", cfg.AHKPort)
	}
	if cfg.TokenTTL != time.Hour {
		t.Errorf("expected default TokenTTL 1h, got %v", cfg.TokenTTL)
	}

	// Verify file was written to disk
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Errorf("expected %s to be created on disk", confPath)
	}

	// 2. Custom INI content
	customContent := `
[Server]
port = 9000
ahk_port = 9001
token_ttl_minutes = 30
max_timestamp_skew_minutes = 20

[AutoPaste]
target_tab_title = mylibrary
overwrite_existing_text = false
auto_paste_on_hover = false
play_tap_sound = false
tooltip_offset_x = 15
tooltip_offset_y = 18

[QRCode]
auto_show_on_refresh = false
auto_hide_seconds = 10
popup_size = 320
`
	if err := os.WriteFile(confPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	cfg2, err := LoadOrCreate(confPath)
	if err != nil {
		t.Fatalf("failed to load custom config: %v", err)
	}

	if cfg2.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg2.Port)
	}
	if cfg2.AHKPort != 9001 {
		t.Errorf("expected AHKPort 9001, got %d", cfg2.AHKPort)
	}
	if cfg2.TokenTTL != 30*time.Minute {
		t.Errorf("expected TokenTTL 30m, got %v", cfg2.TokenTTL)
	}
	if cfg2.MaxSkew != 20*time.Minute {
		t.Errorf("expected MaxSkew 20m, got %v", cfg2.MaxSkew)
	}
	if cfg2.TargetTabTitle != "mylibrary" {
		t.Errorf("expected TargetTabTitle 'mylibrary', got %s", cfg2.TargetTabTitle)
	}
	if cfg2.OverwriteExistingText != false {
		t.Errorf("expected OverwriteExistingText false, got true")
	}
}
