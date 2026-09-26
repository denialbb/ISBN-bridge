# Mobile Client Setup (iOS & Android)

This document describes how mobile scanning works with the Biblios pipeline, including iOS Shortcuts implementation and planned Android support.

---

## 1. iOS Shortcuts Implementation

Scanning from an iPhone uses two lightweight, native Apple Shortcuts that require no third-party App Store applications:

### Shortcut 1: Token Pairing Shortcut
*Used once when starting a scanning session or after token rotation.*

1. **Scan QR/Bar Code**: Prompts camera to scan the centered QR code displayed on the desktop monitor.
2. **Save File**: Writes the scanned token string to local storage (`Shortcuts/biblios_token.txt` in iCloud Drive / On My iPhone), overwriting any prior token.
3. **Notification**: Emits a brief haptic/banner confirmation ("Token saved successfully").

*(Official iCloud shortcut link will be added here)*

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
│  3. Read File (Shortcuts/biblios_token.txt)            │
│     └─ Retrieve secret token string                    │
│                                                        │
│  4. Combine Text Block                                 │
│     └─ Text: [Barcode]|[Formatted Date]|[Token]        │
│                                                        │
│  5. Generate SHA-256 Hash                              │
│     └─ Produces 64-character hexadecimal signature     │
│                                                        │
│  6. Get Contents of URL (HTTP POST)                    │
│     ├─ URL:     http://<PC_IP>:8765/isbn               │
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

*(Official iCloud shortcut link will be added here)*

---

## 2. Setting Up the iPhone

1. **Find PC Local IP Address**:
   - The Go server prints your PC's LAN IP on startup (e.g. `http://192.168.1.107:8765`).
   - Alternatively, open Command Prompt on Windows and run `ipconfig`.
2. **Network Connection**:
   - Ensure the iPhone is connected to the same local Wi-Fi network as the PC.
3. **First-Time Pairing**:
   - Run the **Token Pairing Shortcut** and scan the QR code centered on your monitor.
4. **Scan Books**:
   - Run the **Scanner Shortcut**. The camera scanner opens in a loop. Point it at any book barcode; the ISBN will be validated, authenticated, and pasted onto your PC instantly.

---

## 3. Future Android Support Roadmap

We are designing Android compatibility using the same cryptographic protocol:

- **Protocol Compatibility**: Any client that can generate `SHA-256(ISBN|Timestamp|Token)` and send an HTTP POST request to port `8765` will work seamlessly with the existing Go backend and AutoHotkey client without modifications.
- **Recommended Approaches**:
  1. **HTTP Shortcuts App (F-Droid / Play Store)**: Free, open-source automation app supporting camera barcode triggers, JavaScript hashing (`crypto.subtle`), and HTTP requests.
  2. **Tasker / Automate Script**: Android automation tasks reading stored tokens and sending authenticated POST requests.
  3. **Lightweight PWA / Native APK**: A dedicated, minimal web app running in mobile Chrome using the `BarcodeDetector` API and Web Crypto API.
