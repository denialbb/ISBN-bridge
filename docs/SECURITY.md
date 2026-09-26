# Security Architecture & Cryptographic Verification

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
1. Validate ISBN Format & Checksum (ISBN-10 / ISBN-13)
        │ ── Invalid? ──► HTTP 400 Bad Request
        ▼
2. Verify Timestamp Skew (within +/- max_timestamp_skew_minutes)
        │ ── Expired/Drifted? ──► HTTP 401 Unauthorized
        ▼
3. Compute Expected SHA-256 Hash using Active Server Token
        │
        ▼
4. Constant-Time Hash Comparison (subtle.ConstantTimeCompare)
        │ ── Mismatch? ──► HTTP 401 Unauthorized
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

This guarantees that signature verification takes constant CPU time regardless of how many characters match.

---

## 4. Token Lifecycle & Management

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
