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
token_ttl_minutes = 30

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

# Network interface mode for mobile pairing:
# - auto: automatically prefer USB tethering if plugged in (iOS 172.20.10.x, Android 192.168.42.x); fallback to Wi-Fi/LAN.
# - usb:  exclusively bind/advertise USB tethering interface.
# - lan:  exclusively bind/advertise Wi-Fi or LAN interface.
network_mode = auto

# Optional explicit IP override to advertise in the pairing QR code (leave empty for auto-detection).
server_ip =

[AutoPaste]
# Window/tab title substring required for auto-paste actions (case-insensitive)
target_tab_title = hardcover

# If true, sends Ctrl+A before pasting to replace existing text in the input box
overwrite_existing_text = true

# If true, automatically types/pastes when mouse hovers over an input field (IBeam)
auto_paste_on_hover = true

# Play a short click sound on successful paste
play_tap_sound = true

# Sound sample to play, relative to the client folder
# (e.g. sounds/tap.wav, sounds/ios_tock.wav, sounds/bubble_pop.wav,
# sounds/gentle_chime.wav, sounds/modern_beep.wav,
# sounds/mechanical_click.wav, sounds/windows_navigation.wav)
sound_file = sounds/tap.wav

# Offset of the cursor-tracking tooltip from the mouse pointer in pixels
tooltip_offset_x = 10
tooltip_offset_y = 12

[QRCode]
# Automatically pop up centered QR modal when the token refreshes
auto_show_on_refresh = true

# Duration in seconds before the centered QR modal automatically hides (0 = no auto-hide)
auto_hide_seconds = 180

# Dimensions in pixels of the centered QR code image popup
popup_size = 280

[UI]
# Interface language: auto (detect from OS/browser), en (English), it (Italian).
# More languages are drop-in files — see section 4 below.
language = auto
```

---

## 2. Dynamic Runtime Settings (Taskbar Menu)

The tray icon controls the configuration without editing files manually.
Labels below are the English defaults; Italian ships too and follows the
OS language unless overridden (see section 4).

| Tray Item | Behavior | Persistent |
|---|---|---|
| **Active** | Toggles auto-paste on or off. | Runtime only |
| **Token Expiry** | Submenu: `15m`, `30m`, `1h`, `2h`, `12h`, `24h`. Writes `token_ttl_minutes` to `scanner.conf`, updates the running Go server, and shows the new QR code. | Yes (`scanner.conf`) |
| **Settings → Overwrite text (Ctrl+A)** | Toggles whether existing field text is selected before pasting. | Yes (`scanner.conf`) |
| **Settings → Auto-paste on hover (no click)** | Toggles pasting when the pointer is over an input field. | Yes (`scanner.conf`) |
| **Settings → Audio Feedback** | Submenu with the bundled samples plus **Enable sound**. | Yes (`scanner.conf`) |
| **Settings → Show QR code on token refresh** | Toggles whether the QR pops up automatically when tokens rotate. | Yes (`scanner.conf`) |
| **Settings → Language** | **Auto** plus every installed language. | Yes (`scanner.conf`) |
| **Show centered QR code** | Displays the QR popup (also: middle-click the tray icon). | Action |
| **Show/Hide server console** | Label follows the current console state. | Action |
| **Reset token (New QR)** | Immediately rotates the token and shows the new QR code. | Action |
| **Open scanner.conf** | Opens `scanner.conf` in the default editor. | Action |
| **Clear pending ISBN** | Discards an armed ISBN that has not pasted yet. | Action |
| **Open debug log** | Opens `isbn-bridge-debug.log`. | Action |
| **Exit** | Quits the client (and its managed server). | Action |

---

## 3. Audio Customization

- When `play_tap_sound = true`, the client plays `sound_file` (default `sounds/tap.wav`), resolved relative to the client folder.
- You can replace it with any standard 16-bit PCM WAV file.
- If the file is missing, the client falls back to the default Windows navigation sound (`Windows Navigation Start.wav`).

---

## 4. Languages

English (`en`) and Italian (`it`) ship with the app. `auto` (the default)
picks Italian on Italian-language systems and English everywhere else; the
tray menu switches without a restart. The pairing pages (`/qr`, `/pair`)
use the same setting, then the `?lang=` parameter, then the browser's
`Accept-Language` header.

Adding a language takes two files and no code changes:

1. **Desktop client**: copy `client/lang/en.ini` to
   `client/lang/<code>.ini` (e.g. `fr.ini`). Translate the values, keep
   the keys and the `{1}`/`{2}` placeholders, and fill in `[meta]`
   `language_name` (native name, e.g. `Français`) and `lang_id` (primary
   OS language ID: 9 English, 16 Italian, 12 French, 7 German,
   10 Spanish). Save as UTF-8 with BOM or UTF-16.
2. **Pairing pages**: copy `pkg/server/lang/en.json` to
   `pkg/server/lang/<code>.json` and translate the values (the `{prefix}`
   placeholder stays).

In a packaged install both land in the `lang/` folder next to the
executables (`<code>.ini` for the client, `<code>.json` for the server),
so translators can also add them after install — restart the app and the
new language appears in the tray menu and on the pairing pages. Missing
keys fall back to English.
