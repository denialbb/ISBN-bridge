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
	if cfg.MaxSkew != 15*time.Second {
		t.Errorf("expected default MaxSkew 15s, got %v", cfg.MaxSkew)
	}
	if cfg.RateLimitMax != 2 {
		t.Errorf("expected default RateLimitMax 2, got %d", cfg.RateLimitMax)
	}
	if cfg.RateLimitWindow != 10*time.Second {
		t.Errorf("expected default RateLimitWindow 10s, got %v", cfg.RateLimitWindow)
	}
	if cfg.ReplaySize != 100 {
		t.Errorf("expected default ReplaySize 100, got %d", cfg.ReplaySize)
	}
	if cfg.ReplayTTL != 60*time.Second {
		t.Errorf("expected default ReplayTTL 60s, got %v", cfg.ReplayTTL)
	}
	if !cfg.HideConsole {
		t.Errorf("expected default HideConsole true, got false")
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

func TestHardeningKeys(t *testing.T) {
	tempDir := t.TempDir()
	confPath := filepath.Join(tempDir, "scanner.conf")

	// Seconds key takes precedence over legacy minutes regardless of order.
	content := `
[Server]
max_timestamp_skew_minutes = 20
max_timestamp_skew_seconds = 45
rate_limit_max_requests = 5
rate_limit_window_seconds = 30
replay_cache_size = 50
replay_ttl_seconds = 120
`
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadOrCreate(confPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.MaxSkew != 45*time.Second {
		t.Errorf("expected MaxSkew 45s (seconds key wins), got %v", cfg.MaxSkew)
	}
	if cfg.RateLimitMax != 5 {
		t.Errorf("expected RateLimitMax 5, got %d", cfg.RateLimitMax)
	}
	if cfg.RateLimitWindow != 30*time.Second {
		t.Errorf("expected RateLimitWindow 30s, got %v", cfg.RateLimitWindow)
	}
	if cfg.ReplaySize != 50 {
		t.Errorf("expected ReplaySize 50, got %d", cfg.ReplaySize)
	}
	if cfg.ReplayTTL != 120*time.Second {
		t.Errorf("expected ReplayTTL 120s, got %v", cfg.ReplayTTL)
	}

	// Round-trip through Save preserves the hardening values.
	if err := cfg.Save(confPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	reloaded, err := LoadOrCreate(confPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if reloaded.MaxSkew != 45*time.Second {
		t.Errorf("expected reloaded MaxSkew 45s, got %v", reloaded.MaxSkew)
	}
	if reloaded.RateLimitMax != 5 || reloaded.RateLimitWindow != 30*time.Second {
		t.Errorf("expected reloaded limiter 5/30s, got %d/%v", reloaded.RateLimitMax, reloaded.RateLimitWindow)
	}
	if reloaded.ReplaySize != 50 || reloaded.ReplayTTL != 120*time.Second {
		t.Errorf("expected reloaded replay 50/120s, got %d/%v", reloaded.ReplaySize, reloaded.ReplayTTL)
	}
}
