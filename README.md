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
│  - Token Manager (1h expiry)    │
│  - Terminal & Web QR (/qr)      │
│  - SHA-256 Signature Verifier   │
│  - ISBN Checksum Validator      │
└────────────────┬────────────────┘
                 │
                 │ HTTP POST /paste (localhost only)
                 │ Body: <Normalized ISBN>
                 ▼
┌─────────────────────────────────┐
│     AutoHotkey Client (:8766)   │
│  - Window/Tab Title Matcher     │
│  - I-Beam Cursor Textbox Focus  │
│  - Auto-select & Paste (^A ^V)  │
└─────────────────────────────────┘
```

## Security & Verification

1. **Token Rotation**: Tokens are 48-character cryptographically secure random alphanumeric strings, automatically rotating every hour.
2. **SHA-256 Signature**: The iOS client hashes `ISBN|timestamp|token`. The server computes the expected hash in constant time.
3. **Replay Protection**: The `Timestamp` header is verified against server time (max skew: 15 minutes).
4. **Zero-Secret Exposure**: The token is never transmitted over the wire during ISBN scanning—only the one-time cryptographic hash.
5. **QR Code Pairing**: Whenever a token rotates or on server startup, a scannable QR code is rendered directly in the terminal and on `http://<PC_IP>:8765/qr`.

## Getting Started

### 1. Build & Run the Go Server

```bash
# Run unit tests
go test -v ./...

# Run the server directly
go run ./cmd/server

# Or build binaries (Windows .exe or Linux)
go build -o bin/biblios-server.exe ./cmd/server
```

**Server Flags:**
- `-port` (default: `8765`): HTTP port for iOS Shortcuts.
- `-ahk` (default: `http://127.0.0.1:8766/paste`): Local AutoHotkey listener URL.
- `-token-file` (default: `token.txt`): File to persist active token.
- `-ttl` (default: `1h`): Token validity duration before rotation.
- `-max-skew` (default: `15m`): Maximum timestamp drift allowed.
- `-no-terminal-qr`: Disable printing ANSI QR code to terminal.

### 2. Run the AutoHotkey Client

Run `ISBN scan/ISBN paste.ahk` using AutoHotkey v2:
- Listens on `http://127.0.0.1:8766/paste` for verified ISBNs from Go.
- Shows follow-cursor tooltip and pastes into browser tab matching `hardcover`.

### 3. iOS Shortcuts Setup

#### A. Token Updater Shortcut
- Scans QR code from the server terminal or `http://<PC_IP>:8765/qr`.
- Saves the scanned text to local storage (or a Shortcut variable).

#### B. Scanner Shortcut
1. **Scan QR/Bar Code**
2. **Format Date** (Current Date) -> store in `[Timestamp]`
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
