# System Architecture & Technical Specifications

![Go backend](https://img.shields.io/badge/backend-Go-00ADD8?logo=go&logoColor=white)
![Windows client](https://img.shields.io/badge/client-AutoHotkey_v2-334455?logo=autohotkey&logoColor=white)
![Linux client](https://img.shields.io/badge/client-Python_3.10%2B-3776AB?logo=python&logoColor=white)

This document details the internal architecture, network communication, and module structure of the **ISBN Bridge** pipeline.

---

## 1. System Overview

The system consists of three loosely coupled layers designed for low latency, security, and local execution across Windows and Linux:

```
┌─────────────────────────────────┐
│  Mobile Client (iOS / Android)  │
│  - Barcode / ISBN Camera Loop   │
│  - Token Storage & SHA-256 Sign │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /isbn (over Direct USB Tether or Wi-Fi LAN)
                 │ Header: Authorization: Bearer <SHA256>
                 │ Header: Timestamp: <yyyy-MM-dd HH:mm:ss>
                 │ Body:   <ISBN>
                 ▼
┌─────────────────────────────────┐
│     Go Backend (:8765)          │
│  - Dynamic USB/LAN Resolver     │
│  - CSPRNG Token Manager         │
│  - SHA-256 Constant-Time Auth   │
│  - ISBN-10 / ISBN-13 Checksums  │
│  - Web QR & PNG generators      │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /paste, /qr/show, /qr/hide (localhost only)
                 │ Header: X-ISBN-Bridge-Local: <32-char secret>
                 │ Body: <Normalized ISBN>
                 ▼
┌────────────────────────────────────────────────────────┐
│     Desktop Client (:8766)                             │
│  ├─ Windows: AutoHotkey v2 (Winsock listener, GUI QR)  │
│  └─ Linux: Python (HTTP listener, Tkinter, AppIndicator)│
│     - Focus-targeted auto-typing / paste               │
│     - System tray controls & symbolic theme tinting    │
│     - Centered borderless QR modal popup               │
└────────────────────────────────────────────────────────┘
```

---

## 2. Go Backend Architecture (`pkg/`)

The Go backend acts as the secure gateway between mobile devices (connected via USB cable tethering or local Wi-Fi) and the desktop environment.

### Module Breakdown

| Package | Path | Purpose |
|---|---|---|
| **Interface Resolver** | `pkg/netutil/interfaces.go` | Resolves server IP based on configured mode (`auto`, `usb`, `lan`). Prioritizes IP-over-USB tethering (`172.20.10.x` on iOS `ipheth`, `192.168.4x.x` on Android RNDIS). Hot-plug monitor dynamically detects interface switches. |
| **ISBN Validator** | `pkg/isbn/validator.go` | Single-pass byte-filter normalization (`strings.Builder`) and mathematical checksum verification for ISBN-10 (weighted sum mod 11) and ISBN-13 (alternate 1x/3x mod 10). |
| **Token Manager** | `pkg/auth/token.go` | Generates 48-character cryptographically secure tokens via `crypto/rand.Read`. Manages thread-safe rotation (`sync.RWMutex`), dynamic TTL adjustment, and exports PNG / terminal QR codes. |
| **Signature Verifier** | `pkg/auth/verifier.go` | Computes SHA-256 signature and uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. Enforces timestamp drift limits against replay attacks. |
| **Config Loader** | `pkg/config/config.go` | Reads, writes, and synchronizes options with `scanner.conf`. |
| **HTTP Server** | `pkg/server/server.go` | Exposes REST endpoints, validates payloads up to 16 KB, and forwards verified ISBNs to the local desktop client. |
| **AHK/Desktop Forwarder**| `pkg/server/forwarder.go` | Dispatches HTTP requests to `127.0.0.1:8766` with `X-ISBN-Bridge-Local` authentication and a 2-second timeout. |

### Go Server REST Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/isbn` | Receives scanned ISBN from mobile. Validates checksum & signature, then forwards to AutoHotkey. |
| `GET` | `/qr` | Pairing page with the active token QR code. |
| `GET` | `/qr.png` | Raw PNG of the active token QR code (used by the AHK modal popup). |
| `GET` | `/pair` | Pairing page for phones: auto-launches the iOS shortcut or offers the config JSON. Append `?format=json` for JSON. |
| `GET` | `/` | Redirects to `/qr`. |
| `POST` | `/qr/show` | Triggers the centered AutoHotkey QR popup on the desktop. Localhost only. |
| `POST` | `/token/refresh` | Immediately rotates the active token and triggers the QR popup. Returns the new token as JSON. Localhost only. |
| `POST` | `/token/ttl?minutes=N` | Updates the token TTL and regenerates the active token. Localhost only. |
| `GET` | `/health` | Returns JSON status including expiry state, TTL, and server time. |
| `POST` | `/console/show`, `/console/hide`, `/console/toggle` | Show, hide, or toggle the server console window (called by the desktop client). Localhost only. |
| `GET` | `/console` | Returns whether the server console is visible. Localhost only. |
| `POST` | `/shutdown` | Stops the server (called by the desktop client on exit). Localhost only. |

The AutoHotkey listener on `127.0.0.1:8766` exposes `POST /paste`, `/qr/show`, and `/qr/hide` for the Go forwarder. Every request must carry the `X-ISBN-Bridge-Local` shared secret (written to `local_secret.txt` at server startup); requests without it get `403`.

---

## 3. AutoHotkey Desktop Client Architecture (`client/`)

The desktop client is written in AutoHotkey v2 and organized into modular classes under `client/lib/`:

```
client/
├── main.ahk             # Entrypoint & hotkey wiring
└── lib/
    ├── config.ahk       # AppConfig: scanner.conf parser & writer
    ├── logger.ahk       # Logger: timestamped logging to isbn-bridge-debug.log
    ├── paste.ahk        # PasteEngine: auto-paste & hover detection
    ├── server.ahk       # HttpListener: Winsock non-blocking HTTP server
    ├── sound.ahk        # SoundManager: tap sound on paste
    ├── tooltip.ahk      # FollowToolTip: 30 Hz mouse-following UI overlay
    ├── tray.ahk         # TrayManager: taskbar context menu & TTL submenu
    └── ui_qr.ahk        # QRModal: centered borderless GUI popup
```

### Key Subsystems

1. **Winsock Non-Blocking Listener (`lib/server.ahk`)**:
   - Uses native `ws2_32.dll` system calls (`socket`, `ioctlsocket(FIONBIO)`, `bind`, `listen`, `recv`).
   - Polls active sockets via bound method timers (`ObjBindMethod`) at 40 Hz (25 ms period) without blocking the Windows message pump.
   - Binds port `8766` on `127.0.0.1` only, so only the local machine can reach it.

2. **Paste & Hover Engine (`lib/paste.ahk`)**:
   - **Hover Insertion**: When the pointer is over an `IBeam` cursor in the target window (e.g. `hardcover`), the ISBN is pasted with no click.
   - **Overwrite Mode**: Sends `Ctrl+A` before pasting to replace existing placeholder text when configured.
   - **Clipboard Use**: Puts the ISBN on the clipboard for pasting. The previous clipboard contents are not restored, so copy anything you need before scanning.

3. **Follow Tooltip (`lib/tooltip.ahk`)**:
   - Smoothly tracks mouse movement at 30 Hz (33 ms interval).
   - Offsets tooltip close to the cursor tip (`+10, +12`) using Win32 `SetWindowPos` (`SWP_NOSIZE | SWP_NOACTIVATE | SWP_NOZORDER`).

4. **Centered QR Modal (`lib/ui_qr.ahk`)**:
   - Borderless, frameless GUI window appearing in the exact center of the monitor.
   - Displays live token TTL status and automatically closes on scan, click, or `ESC`.

---

## 4. Linux Desktop Client Architecture (`client_linux/`)

The Linux desktop client is implemented in Python (3.10+) with zero heavy web dependencies, designed for low-memory footprint and native integration on Wayland and X11:

```
client_linux/
├── src/client_linux/
│   ├── main.py             # Entrypoint, lifecycle, and signal handling
│   ├── config.py           # AppConfig: scanner.conf parser & writer
│   ├── logger.py           # Logger: sanitized file logging to isbn-bridge-debug.log
│   ├── isbn.py             # Pure-Python ISBN-10 / ISBN-13 validator & normalizer
│   ├── paste.py            # PasteEngine: auto-typing via wtype, xdotool, or uinput
│   ├── server.py           # HttpListener: standard library HTTP server with secret check
│   ├── server_manager.py   # Spawns, monitors, and stops Go backend server
│   ├── sound.py            # SoundManager: audio playback via paplay, aplay, or pw-play
│   ├── tooltip.py          # Notifier: desktop notifications via notify-send
│   ├── tray.py             # TrayManager: AyatanaAppIndicator / SNI with Omarchy tinting
│   └── ui_qr.py            # QRModal: Tkinter borderless centered modal popup
```

### Key Subsystems

1. **System Tray & Omarchy Theme Integration (`tray.py`)**:
   - Uses `AyatanaAppIndicator3` (or `AppIndicator3` fallback) to provide a desktop tray indicator across Hyprland/Omarchy, Sway, KDE Plasma, and GNOME.
   - Discovers `isbn-bridge-symbolic.png` so modern status bars (such as Omarchy's Quickshell bar) recognize it as a freedesktop symbolic icon and automatically tint it with the theme foreground color (`colorizationColor`).
   - Hooks into the `activate` signal: left-clicking the tray icon immediately displays the centered QR pairing modal on the focused monitor.

2. **Paste & Keystroke Injection Engine (`paste.py`)**:
   - Wayland: Dispatches typing via `wtype` or kernel `/dev/uinput`, reading active window titles over Hyprland or Sway IPC sockets.
   - X11: Falls back to `xdotool` and `xclip`.
   - Verifies target tab title before emission to avoid typing into unintended applications.

3. **Centered QR Pairing Modal (`ui_qr.py`)**:
   - Pure Tkinter borderless popup (`overrideredirect(True)`), styled with dark background and brand accent.
   - Inspects monitor geometry dynamically via `hyprctl monitors -j` or screen dimensions to center the modal on the currently focused display.
   - Automatically re-renders in-memory when tokens rotate or the network interface changes.
