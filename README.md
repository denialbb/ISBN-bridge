# Biblios Scanner & Auto-Paste

High-performance, secure ISBN scanner pipeline connecting iOS Shortcuts, a Go validation & token server, and an AutoHotkey auto-pasting desktop client.

## Architecture

```
┌─────────────────────────────────┐
│     iPhone Shortcuts Client     │
│  - Scanner (Camera + Barcode)   │
│  - Token QR Scanner (Storage)   │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /isbn
                 │ Header: Authorization: Bearer <SHA256>
                 │ Header: Timestamp: <formatted_date>
                 │ Body:   <ISBN>
                 ▼
┌─────────────────────────────────┐
│     Go Server (:8765)           │
│  - Token Manager (Config TTL)   │
│  - Terminal & Web QR (/qr)      │
│  - SHA-256 Signature Verifier   │
│  - ISBN Checksum Validator      │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /paste, /qr/show, /qr/hide (localhost)
                 │ Body: <Normalized ISBN>
                 ▼
┌─────────────────────────────────┐
│     AutoHotkey Client (:8766)   │
│  - Centered Frameless QR Popup  │
│  - Zero-Click Hover Auto-Paste  │
│  - Tactile Tap Audio Feedback   │
│  - Taskbar Icon Menu & Options  │
└─────────────────────────────────┘
```

## Features

1. **Seamless Centered QR Popup**:
   - Appears automatically in the exact center of the screen on server startup or token rotation.
   - Clean, modern, borderless card with token expiration info.
   - Dismisses automatically as soon as an iPhone scans an ISBN, or by clicking / pressing `ESC`.
2. **Unified Configuration (`scanner.conf`)**:
   - Single file containing all ports, timings, audio, visual, and pasting options.
   - Automatically loaded and updated by both the Go server and AutoHotkey.
3. **Taskbar Tray Options**:
   - **Scadenza token**: Submenu allowing on-the-fly selection of token lifespan (15m, 30m, 1h, 2h, 12h, 24h) with live radio checkmarks and automatic Go server sync.
   - **Sovrascrivi testo (Ctrl+A)**: Checkbox to toggle auto-select before paste.
   - **Auto-incolla al passaggio (senza clic)**: Inserts ISBN automatically if mouse is already hovering over an input field.
   - **Suono al tocco (Tap)**: Crisp tactile audio feedback.
   - **Mostra QR code al centro**: Opens the centered QR overlay on demand.
4. **Security & Cryptographic Verification**:
   - iOS generates `SHA256(ISBN|timestamp|token)`.
   - Go server computes and validates the hash in constant time (`subtle.ConstantTimeCompare`).
   - Timestamps protect against replay attacks.

## Unified Configuration (`scanner.conf`)

```ini
[Server]
port = 8765
ahk_port = 8766
token_ttl_minutes = 60
max_timestamp_skew_minutes = 15

[AutoPaste]
target_tab_title = hardcover
overwrite_existing_text = true
auto_paste_on_hover = true
play_tap_sound = true
tooltip_offset_x = 10
tooltip_offset_y = 12

[QRCode]
auto_show_on_refresh = true
auto_hide_seconds = 45
popup_size = 280
```

## Getting Started

### 1. Build & Run the Go Server

```bash
# Run unit tests
go test -v ./...

# Run the server directly
go run ./cmd/server

# Or run the native Windows binary
.\bin\biblios-server.exe
```

### 2. Run the AutoHotkey Client

Run `ISBN scan/ISBN paste.ahk` using AutoHotkey v2:
```powershell
& "ISBN scan\ISBN paste.ahk"
```

### 3. iOS Shortcuts Setup

#### A. Token Updater Shortcut
- Scans the centered QR code shown on your monitor (or from `http://<PC_IP>:8765/qr`).
- Saves the token text to local storage.

#### B. Scanner Shortcut
1. **Scan QR/Bar Code**
2. **Format Date** (`Current Date`) -> `[Timestamp]`
3. **Get File from Storage** (retrieve active token) -> `[Token]`
4. **Text block**: `[QR/Bar Code]|[Timestamp]|[Token]`
5. **Generate SHA-256**: Hash the text block with SHA-256.
6. **Get Contents of URL**:
   - URL: `http://<PC_IP>:8765/isbn`
   - Method: `POST`
   - Headers:
     - `Authorization`: `Bearer [SHA-256 Hash]`
     - `Timestamp`: `[Timestamp]`
   - Request Body: `File` -> `[QR/Bar Code]` (as Text)
