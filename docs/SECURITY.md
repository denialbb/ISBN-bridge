# Security Architecture & Cryptographic Verification

![SHA-256 auth](https://img.shields.io/badge/auth-SHA--256-success)
![Replay protection](https://img.shields.io/badge/replay-single--use-blue)
![Rate limited](https://img.shields.io/badge/ratelimit-2req%2F10s-orange)

This document details the threat model, authentication protocol, and cryptographic measures implemented in the ISBN Bridge system.

---

## 1. Threat Model

Because mobile devices (iPhones, Android phones) communicate with the desktop server over local Wi-Fi networks (LAN), the system protects against:

1. **Unauthorized LAN Injections**: Rogue devices or automated tools on the local network attempting to inject arbitrary text or keypresses into the user's desktop session.
2. **Eavesdropping / Packet Sniffing**: Passive eavesdroppers observing unencrypted Wi-Fi traffic.
3. **Replay Attacks**: Attackers capturing a previously valid ISBN submission and replaying it at a later time.
4. **Timing Attacks**: Side-channel attacks attempting to guess token characters based on comparison latency.

---

## 2. Authentication Protocol

Instead of sending the raw secret token over the network with every scan, the client sends a cryptographic proof of knowledge:

### Signature Construction

```
Signature = SHA-256( raw_isbn + "|" + timestamp + "|" + token )
```

- **`raw_isbn`**: The exact barcode string scanned by the camera (e.g. `9780306406157`).
- **`timestamp`**: Formatted date string generated on the mobile device at scan time (`yyyy-MM-dd HH:mm:ss`).
- **`token`**: The active 48-character secret token saved on the mobile device.

### HTTP Request Specification

```http
POST /isbn HTTP/1.1
Host: 192.168.1.100:8765
Authorization: Bearer <64_character_hex_sha256>
Timestamp: 2026-09-26 18:45:00
Content-Type: text/plain; charset=utf-8

9780306406157
```

---

## 3. Server Verification Pipeline

When a request arrives at `POST /isbn`, the Go server executes a strict verification sequence:

```
[Incoming Request]
        │
        ▼
0. Per-IP Rate Limit (sliding window, default 2 req / 10s)
        │ ── Exceeded? ──► HTTP 429 Too Many Requests
        ▼
1. Validate ISBN Format & Checksum (ISBN-10 / ISBN-13)
        │ ── Invalid? ──► HTTP 400 Bad Request
        ▼
2. Verify Timestamp Presence, Format & Skew (within +/- max_timestamp_skew_seconds, default 15s)
        │ ── Missing/Unparseable/Drifted? ──► HTTP 401 Unauthorized
        ▼
3. Compute Expected SHA-256 Hash using Active Server Token
        │
        ▼
4. Constant-Time Hash Comparison (subtle.ConstantTimeCompare)
        │ ── Mismatch? ──► HTTP 401 Unauthorized
        ▼
4b. Single-Use Signature Check (replay cache, default 100 entries / 60s TTL)
        │ ── Already Used? ──► HTTP 409 Conflict
        ▼
5. Forward to AutoHotkey Localhost Listener (:8766)
        │ ── AHK Offline? ──► HTTP 502 Bad Gateway
        ▼
[HTTP 200 OK Delivered to Mobile Device]
```

### Constant-Time Verification

To eliminate timing side-channel attacks, hash validation uses Go's standard library `crypto/subtle.ConstantTimeCompare`:

```go
expectedHash := sha256Hex(isbn + "|" + timestamp + "|" + activeToken)
if subtle.ConstantTimeCompare([]byte(clientHash), []byte(expectedHash)) != 1 {
    return ErrInvalidSignature
}
```

The hash comparison itself runs in constant time for equal-length inputs; it does not make the whole request path constant-time.

---

## 4. LAN & Physical Link Hardening

### 4.0 Physical USB Link Isolation (Recommended)

When connected via direct USB cable tethering (`172.20.10.x` for iPhone, `192.168.4x.x` for Android), communication is physically bounded to the point-to-point USB cable interface. It completely bypasses the local Wi-Fi router and network infrastructure, eliminating wireless eavesdropping, unauthorized LAN injections, and Wi-Fi packet sniffing risks without requiring custom certificates.

### 4.1 Plain HTTP on Trusted LANs

When operating over Wi-Fi, HTTPS is not enabled by default; opt-in TLS is tracked in [#1](https://github.com/denialbb/ISBN-bridge/issues/1). On open or shared Wi-Fi networks, users should prefer direct USB cable tethering. When on Wi-Fi, the following defense-in-depth measures protect the session:

### 4.2 Tight Timestamp Window (±15 seconds)

`max_timestamp_skew_seconds` (default: `15`). A request whose `Timestamp`
header is missing, unparseable, or outside the window is rejected with
`401`. A sniffed request therefore stays usable for seconds only. The legacy
`max_timestamp_skew_minutes` key is still honored when the seconds key is
absent. Both the phone and the PC are normally NTP-synced to within a
second, so the tight default is safe; raise it if you see false `401`s.

### 4.3 Single-Use Signatures (Replay Protection)

The server remembers the last `replay_cache_size` (default: `100`) accepted
signatures for `replay_ttl_seconds` (default: `60`). Replaying a captured
request returns `409 Conflict` and never reaches the desktop. Purely
in-memory; no persistent storage.

### 4.4 Per-IP Rate Limiting

`POST /isbn` is limited to `rate_limit_max_requests` (default: `2`) per
`rate_limit_window_seconds` (default: `10`) sliding window per source IP.
Excess requests get `429 Too Many Requests`. Tune to taste: `0` disables the
limiter. Health, QR, and pairing endpoints are not limited.

### 4.5 Focus and Window Checks (Desktop Clients)

- **Windows (AutoHotkey)**: The `PasteEngine` only emits keystrokes when the focused window matches `target_tab_title` **and** the cursor is an IBeam (text field). The check runs three times: at click time, 100 ms later in `OnDeferredClick`, and immediately before `SendInput` in `PasteNow` — so a window switch in the arming gap aborts the paste.
- **Linux (Python)**: Active window titles are verified before typing via Hyprland IPC (`hyprctl activewindow -j`), Sway IPC, or `xdotool getactivewindow`. If the focused window does not match `target_tab_title`, keystroke injection is aborted.

### 4.6 Local Control Plane & Zero Log Leakage

The endpoints the desktop client drives (`/token/refresh`, `/token/ttl`,
`/shutdown`, `/console/*`, `/qr/show`) accept loopback requests only
(`127.0.0.1`, `::1`); LAN callers get `403`. The phone only needs `/isbn`,
`/qr*`, `/pair`, and `/health`, which stay reachable over LAN or USB tether.

The hop from Go to the desktop client (`127.0.0.1:8766`, bound to localhost) requires
a separate credential: the server generates a 32-character cryptographically secure
secret at startup, stores it in `local_secret.txt` (`0600`) next to the token file, and
sends it as `X-ISBN-Bridge-Local` on every forward. The client re-reads the file
per request, allowing seamless server restarts.

**Zero Log Leakage Policy**:
- Server standard output and debug logs never output secret tokens, token prefixes, or pairing URLs with sensitive parameters.
- Client HTTP access logs strip query parameters so secrets are never written to `isbn-bridge-debug.log`.
- Invalid request payloads are never echoed into logs, preventing log injection or accidental sensitive data leakage.
- Temporary QR images are loaded directly in memory or deleted immediately after display.

---

## 5. Token Lifecycle & Management

### CSPRNG Generation
Tokens are generated using cryptographically secure pseudorandom numbers from the operating system (`crypto/rand.Read`):
- Length: **48 characters**
- Character Set: `[a-zA-Z0-9]` (62 possible characters)
- Entropy: ~285 bits of cryptographic entropy, preventing brute-force guessing.

### Automatic Rotation
Tokens rotate automatically when their time-to-live (TTL) expires (default: 60 minutes, configurable via `scanner.conf` or the tray menu).

### Instant Desktop Pairing
Upon startup or token rotation:
1. The desktop client displays a centered, frameless QR code containing the new token.
2. The user points the mobile camera to update the stored token in one tap.
3. Once the mobile shortcut sends its next scan, the desktop QR code dismisses automatically.
