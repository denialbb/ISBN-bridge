# Configuration Guide (`scanner.conf`)

![Single file config](https://img.shields.io/badge/config-scanner.conf-blue)

ISBN Bridge uses a single unified configuration file, [`scanner.conf`](../scanner.conf), shared between the Go backend server and the AutoHotkey desktop client.

---

## 1. Configuration Reference

```ini
[Server]
# Port for Go HTTP server (receives requests from mobile devices)
port = 8765

# Port for local AutoHotkey listener (receives local commands from Go server)
ahk_port = 8766

# Token expiration in minutes. The token automatically rotates when expired.
# Options available via tray menu: 15, 30, 60, 120, 720, 1440.
token_ttl_minutes = 60

# Maximum allowed clock drift between iPhone and PC in seconds.
# Tight window: a sniffed request stays usable for seconds only.
# (Legacy max_timestamp_skew_minutes is honored if this key is absent.)
max_timestamp_skew_seconds = 15

# Per-IP rate limit for POST /isbn (LAN anti-spam). 0 disables.
rate_limit_max_requests = 2
rate_limit_window_seconds = 10

# Single-use signature replay protection. 0 disables.
replay_cache_size = 100
replay_ttl_seconds = 60

[AutoPaste]
# Window/tab title substring required for auto-paste actions (case-insensitive)
target_tab_title = hardcover

# If true, sends Ctrl+A before pasting to replace existing text in the input box
overwrite_existing_text = true

# If true, automatically types/pastes when mouse hovers over an input field (IBeam)
auto_paste_on_hover = true

# Play tactile audio click on successful paste
play_tap_sound = true

# Offset of the cursor-tracking tooltip from the mouse pointer in pixels
tooltip_offset_x = 10
tooltip_offset_y = 12

[QRCode]
# Automatically pop up centered QR modal when the token refreshes
auto_show_on_refresh = true

# Duration in seconds before the centered QR modal automatically hides (0 = no auto-hide)
auto_hide_seconds = 45

# Dimensions in pixels of the centered QR code image popup
popup_size = 280
```

---

## 2. Dynamic Runtime Settings (Taskbar Menu)

The AutoHotkey taskbar tray icon provides live control over the configuration without editing files manually:

| Tray Item | Behavior | Persistent |
|---|---|---|
| **Attivo** | Toggles overall Auto-Paste listening on or off. | Runtime only |
| **Scadenza token** | Submenu with options: `15m`, `30m`, `1h`, `2h`, `12h`, `24h`. Updates `token_ttl_minutes` in `scanner.conf`, updates the running Go server, and pops up the new QR code. | Yes (`scanner.conf`) |
| **Sovrascrivi testo (Ctrl+A)** | Toggles whether existing field text is selected before pasting. | Yes (`scanner.conf`) |
| **Auto-incolla al passaggio** | Toggles zero-click insertion when mouse pointer is over an input field. | Yes (`scanner.conf`) |
| **Suono al tocco (Tap)** | Toggles the tactile audio feedback upon paste. | Yes (`scanner.conf`) |
| **Mostra QR code al cambio** | Toggles whether QR pops up automatically when tokens rotate. | Yes (`scanner.conf`) |
| **Mostra QR code al centro** | Manually displays the centered QR popup at any time. | Action |
| **Reset token (Nuovo QR)** | Immediately rotates the token and shows the new QR code. | Action |
| **Apri scanner.conf** | Opens `scanner.conf` directly in the default editor. | Action |
| **Apri log debug** | Opens `isbn-bridge-debug.log` to inspect real-time events. | Action |

---

## 3. Audio Customization

- When `play_tap_sound = true`, the system plays the configured file from `client/sounds/` (default `sounds/tap.wav`), resolved relative to the client folder.
- You can replace it with any standard 16-bit PCM WAV audio file of your choice.
- If the file is missing, the client automatically falls back to the default Windows navigation sound (`Windows Navigation Start.wav`).
