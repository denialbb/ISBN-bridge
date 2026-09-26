package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config represents all tunable settings across the Go server and AutoHotkey script.
type Config struct {
	mu sync.RWMutex

	// Server settings
	Port     int           // Port for incoming iOS shortcut requests (default: 8765)
	AHKPort  int           // Port for local AutoHotkey paste listener (default: 8766)
	TokenTTL time.Duration // Token lifespan before auto-rotation (default: 60m)
	MaxSkew  time.Duration // Allowed clock drift for iOS request timestamps (default: 15m)

	// AutoPaste settings
	TargetTabTitle        string // Browser window/tab title filter (default: "hardcover")
	OverwriteExistingText bool   // Whether to select-all (Ctrl+A) before pasting (default: true)
	AutoPasteOnHover      bool   // Insert immediately if mouse is hovering an input field (default: true)
	PlayTapSound          bool   // Play tactile audio feedback upon pasting (default: true)
	TooltipOffsetX        int    // Horizontal cursor offset for tooltips (default: 10)
	TooltipOffsetY        int    // Vertical cursor offset for tooltips (default: 12)

	// QR Code Display settings
	QRAutoShowOnRefresh bool // Automatically show QR code on screen when token rotates (default: true)
	QRAutoHideSeconds   int  // Seconds before hiding QR if unscanned, 0 to disable (default: 45)
	QRPopupSize         int  // Width/height in pixels of the centered QR popup (default: 280)

	// UI settings
	Language  string // Interface language: "auto", "it", "en" (default: "auto")
	SoundFile string // Sound sample file (default: "tap.wav")

	FilePath string
}

// Default returns a Config populated with production defaults.
func Default() *Config {
	return &Config{
		Port:                  8765,
		AHKPort:               8766,
		TokenTTL:              60 * time.Minute,
		MaxSkew:               15 * time.Minute,
		TargetTabTitle:        "hardcover",
		OverwriteExistingText: true,
		AutoPasteOnHover:      true,
		PlayTapSound:          true,
		SoundFile:             "tap.wav",
		TooltipOffsetX:        10,
		TooltipOffsetY:        12,
		QRAutoShowOnRefresh:   true,
		QRAutoHideSeconds:     45,
		QRPopupSize:           280,
		Language:              "auto",
	}
}

// LoadOrCreate loads configuration from the specified INI path. If the file is missing, it creates it with default values.
func LoadOrCreate(path string) (*Config, error) {
	cfg := Default()
	cfg.FilePath = path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := cfg.Save(path); err != nil {
			return nil, fmt.Errorf("failed to create default config at %s: %w", path, err)
		}
		return cfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "port":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.Port = n
			}
		case "ahk_port":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.AHKPort = n
			}
		case "token_ttl_minutes":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.TokenTTL = time.Duration(n) * time.Minute
			}
		case "max_timestamp_skew_minutes":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxSkew = time.Duration(n) * time.Minute
			}
		case "target_tab_title":
			cfg.TargetTabTitle = val
		case "overwrite_existing_text":
			cfg.OverwriteExistingText = parseBool(val, true)
		case "auto_paste_on_hover":
			cfg.AutoPasteOnHover = parseBool(val, true)
		case "play_tap_sound":
			cfg.PlayTapSound = parseBool(val, true)
		case "tooltip_offset_x":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.TooltipOffsetX = n
			}
		case "tooltip_offset_y":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.TooltipOffsetY = n
			}
		case "auto_show_on_refresh", "qr_auto_show_on_refresh":
			cfg.QRAutoShowOnRefresh = parseBool(val, true)
		case "auto_hide_seconds", "qr_auto_hide_seconds":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.QRAutoHideSeconds = n
			}
		case "popup_size", "qr_popup_size":
			if n, err := strconv.Atoi(val); err == nil && n > 50 {
				cfg.QRPopupSize = n
			}
		case "language":
			cfg.Language = strings.ToLower(val)
		case "sound_file":
			cfg.SoundFile = val
		}
	}

	return cfg, scanner.Err()
}

// SetTokenTTL updates the TTL in-memory and saves to disk.
func (c *Config) SetTokenTTL(ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.TokenTTL = ttl
	if c.FilePath != "" {
		return c.Save(c.FilePath)
	}
	return nil
}

// Save serializes the configuration to disk in INI format with clear commentary.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	content := fmt.Sprintf(`# ==============================================================================
# BIBLIOS SCANNER & AUTO-PASTE CONFIGURATION (scanner.conf)
# Edit values below to customize ports, timings, audio, and visual behavior.
# Both Go Server and AutoHotkey reload/use these settings.
# ==============================================================================

[Server]
# Port listening for incoming iOS Shortcut requests
port = %d

# Port of the local AutoHotkey paste receiver
ahk_port = %d

# Token expiration duration in minutes before automatic regeneration (e.g. 15, 30, 60, 120)
token_ttl_minutes = %d

# Maximum allowed clock difference (in minutes) between iPhone and PC
max_timestamp_skew_minutes = %d


[AutoPaste]
# Window or browser tab title required for auto-paste (leave empty to paste into any active window)
target_tab_title = %s

# Automatically select all text in the textbox (Ctrl+A) before pasting
overwrite_existing_text = %t

# Automatically insert ISBN without clicking if mouse is already hovering a textbox
auto_paste_on_hover = %t

# Play a tactile tap audio effect upon successful paste
play_tap_sound = %t

# Horizontal and vertical pixel distance between the mouse cursor and floating tooltips
tooltip_offset_x = %d
tooltip_offset_y = %d


[QRCode]
# Automatically pop up the QR code in the center of the screen when token is refreshed
auto_show_on_refresh = %t

# Number of seconds before the centered QR popup auto-dismisses (0 to keep open until scanned/closed)
auto_hide_seconds = %d

# Size in pixels of the centered QR code image
popup_size = %d
`,
		c.Port,
		c.AHKPort,
		int(c.TokenTTL.Minutes()),
		int(c.MaxSkew.Minutes()),
		c.TargetTabTitle,
		c.OverwriteExistingText,
		c.AutoPasteOnHover,
		c.PlayTapSound,
		c.TooltipOffsetX,
		c.TooltipOffsetY,
		c.QRAutoShowOnRefresh,
		c.QRAutoHideSeconds,
		c.QRPopupSize,
	)

	return os.WriteFile(path, []byte(content), 0644)
}

func parseBool(val string, defaultVal bool) bool {
	lower := strings.ToLower(strings.TrimSpace(val))
	switch lower {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultVal
	}
}
