# Mobile Client Setup (iOS & Android)

![iOS Shortcuts](https://img.shields.io/badge/iOS-Shortcuts-black?logo=apple&logoColor=white)
![Android HTTP Shortcuts](https://img.shields.io/badge/Android-HTTP_Shortcuts-green?logo=android&logoColor=white)

This document describes mobile scanning: the iOS Shortcuts flow and the Android HTTP Shortcuts flow.

---

## 1. iOS Shortcuts Implementation

Scanning from an iPhone uses two Apple Shortcuts. No third-party App Store apps needed:

### Shortcut 1: Automatic Pairing Shortcut ("Pair ISBN Bridge")
*Used once when starting a scanning session, changing Wi-Fi networks, or after token rotation.*

1. **Receive Input**: Receives JSON config payload (`{"url":"...","token":"..."}`) passed directly from the browser or QR scanner.
2. **Save File**: Writes the configuration dictionary directly to `Shortcuts/isbn_bridge_config.json` in iCloud Drive / On My iPhone (Overwrite: `true`).
3. **Notification**: Vibrates device and shows confirmation ("Paired with PC at [url]").
4. **Auto-Launch Scanner (Required)**: Calls action **"Run Shortcut"** targeting your Scanner Shortcut with *"Wait Until Finished"* (or *"Wait for Response"*) turned **OFF**, followed by **"Exit Shortcut"**. Pairing then hands off instantly: the Pair shortcut terminates and the book scanner opens directly. Leaving wait enabled leaves Pair lingering until the scanner closes.

> **Camera Pairing**: Point your iPhone camera at the QR code on your PC. Tap the link, tap **Open in Shortcuts**, and the scanner opens. No IP lookup or typing needed.

*Download: [Pair ISBN Bridge](https://www.icloud.com/shortcuts/2cc219d6251f46d69ce5d6d3f4ce8cc8)*

---

### Shortcut 2: Continuous Scanner & Sender Shortcut
*The main scanning loop for scanning books.*

```
┌────────────────────────────────────────────────────────┐
│                      Repeat Loop                       │
│                                                        │
│  1. Scan Barcode (EAN-13 / ISBN)                       │
│     └─ If cancelled: Exit Shortcut                     │
│                                                        │
│  2. Format Date (Current Date)                         │
│     └─ Format: "yyyy-MM-dd HH:mm:ss"                   │
│                                                        │
│  3. Read File (Shortcuts/isbn_bridge_config.json)      │
│     ├─ Get Dictionary Value "url"   -> [Server URL]    │
│     └─ Get Dictionary Value "token" -> [Token]         │
│                                                        │
│  4. Combine Text Block                                 │
│     └─ Text: [Barcode]|[Formatted Date]|[Token]        │
│                                                        │
│  5. Generate SHA-256 Hash                              │
│     └─ Produces 64-character hexadecimal signature     │
│                                                        │
│  6. Get Contents of URL (HTTP POST)                    │
│     ├─ URL:     [Server URL]/isbn                      │
│     ├─ Method:  POST                                   │
│     ├─ Headers:                                        │
│     │   ├─ Authorization: Bearer [SHA-256 Hash]        │
│     │   └─ Timestamp:     [Formatted Date]             │
│     └─ Body:    [Barcode] (Plain Text)                 │
│                                                        │
│  7. Vibrate / Audio Feedback                           │
│     └─ Confirm delivery before looping to next scan    │
│                                                        │
└────────────────────────────────────────────────────────┘
```

*Download: [ISBN Bridge](https://www.icloud.com/shortcuts/46434ad59d2b4bff92f8a2460bee9207)*

---

## 2. Setting Up the iPhone

### Option A: Direct USB Cable (Recommended for lowest latency & isolated networks)
1. Connect your iPhone to your PC (Windows or Linux) with a Lightning or USB-C cable.
2. On your iPhone: go to **Settings > Personal Hotspot** and turn it **ON** (select *"USB Only"* when prompted).
   - *Note: Cellular data can be disabled if desired; iPhone and PC communicate over the cable via local `172.20.10.x` without needing internet.*
3. ISBN Bridge automatically detects the USB connection, switches the pairing address to the cable interface, and updates the desktop QR code popup.
4. Point your iPhone camera at the centered QR code to pair over the cable.

### Option B: Local Wi-Fi
1. Ensure the iPhone is connected to the same local Wi-Fi network as your PC.
2. Start the Go server and desktop client on your PC.
3. Point your iPhone camera at the centered QR code on your monitor.
4. Tap the link, then tap **Open in Shortcuts** to save the configuration automatically.

### Scanning:
Run the **Scanner Shortcut**. The camera opens in a loop. Point it at any book barcode; the ISBN is signed and pasted to your PC.

---

## 3. Android Support (HTTP Shortcuts)

Android uses the same cryptographic protocol — no server changes needed:

- **Protocol Compatibility**: Any client that can generate `SHA-256(ISBN|Timestamp|Token)` and POST it to port `8765` works with the Go backend and AutoHotkey client.
- **Recommended app**: [HTTP Shortcuts](https://f-droid.org/packages/ch.rmy.android.http_shortcuts/) (free, open-source) with [Binary Eye](https://f-droid.org/packages/de.markusfisch.android.binaryeye/) as the barcode scanner backend.
- **Setup**: [`mobile/android/SETUP.md`](../mobile/android/SETUP.md) — pair once via the `/pair` page (copy `url` + `token` into global variables), import the flow, scan.
- **Scanner script**: [`mobile/android/scanner.js`](../mobile/android/scanner.js) — pre-execution script verified against the Go verifier (syntax, stubbed composition run, live 401/502/409 matrix).
