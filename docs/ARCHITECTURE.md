# System Architecture & Technical Specifications

![Go backend](https://img.shields.io/badge/backend-Go-00ADD8?logo=go&logoColor=white)
![AHK client](https://img.shields.io/badge/client-AutoHotkey_v2-334455?logo=autohotkey&logoColor=white)

This document details the internal architecture, network communication, and module structure of the **ISBN Bridge** pipeline.

---

## 1. System Overview

The system consists of three loosely coupled layers designed for low latency, security, and local execution:

```
┌─────────────────────────────────┐
│     Mobile Client (iOS)         │
│  - Barcode / ISBN Camera Loop   │
│  - Token Storage & SHA-256 Sign │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /isbn (over LAN)
                 │ Header: Authorization: Bearer <SHA256>
                 │ Header: Timestamp: <yyyy-MM-dd HH:mm:ss>
                 │ Body:   <ISBN>
                 ▼
┌─────────────────────────────────┐
│     Go Backend (:8765)          │
│  - CSPRNG Token Manager         │
│  - SHA-256 Constant-Time Auth   │
│  - ISBN-10 / ISBN-13 Checksums  │
│  - Web QR & PNG generators      │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /paste, /qr/show, /qr/hide (localhost only)
                 │ Body: <Normalized ISBN>
                 ▼
┌─────────────────────────────────┐
│     AutoHotkey Client (:8766)   │
│  - Non-blocking Winsock Server  │
│  - Centered QR Overlay │
│  - Hover-detection & Paste      │
│  - Tray Menu & Config Sync      │
└─────────────────────────────────┘
```

---

## 2. Go Backend Architecture (`pkg/`)

The Go backend acts as the secure gateway between mobile devices on the local Wi-Fi network and the desktop environment.

### Module Breakdown

| Package | Path | Purpose |
|---|---|---|
| **ISBN Validator** | `pkg/isbn/validator.go` | Single-pass byte-filter normalization (`strings.Builder`) and mathematical checksum verification for ISBN-10 (weighted sum mod 11) and ISBN-13 (alternate 1x/3x mod 10). |
| **Token Manager** | `pkg/auth/token.go` | Generates 48-character cryptographically secure tokens via `crypto/rand.Read`. Manages thread-safe rotation (`sync.RWMutex`), dynamic TTL adjustment, and exports PNG / terminal QR codes. |
| **Signature Verifier** | `pkg/auth/verifier.go` | Computes SHA-256 signature and uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. Enforces timestamp drift limits against replay attacks. |
| **Config Loader** | `pkg/config/config.go` | Reads, writes, and synchronizes options with `scanner.conf`. |
| **HTTP Server** | `pkg/server/server.go` | Exposes REST endpoints, validates payloads up to 16 KB, and forwards verified ISBNs to the local AutoHotkey client. |
| **AHK Forwarder** | `pkg/server/forwarder.go` | Dispatches HTTP requests to `127.0.0.1:8766` with a strict 2-second timeout. |

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
