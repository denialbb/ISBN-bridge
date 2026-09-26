# Mobile Client Setup (iOS & Android)

![iOS Shortcuts](https://img.shields.io/badge/iOS-Shortcuts-black?logo=apple&logoColor=white)
![Android planned](https://img.shields.io/badge/Android-planned-lightgrey?logo=android&logoColor=white)

This document describes how mobile scanning works with the ISBN Bridge pipeline, including iOS Shortcuts implementation and planned Android support.

---

## 1. iOS Shortcuts Implementation

Scanning from an iPhone uses two lightweight, native Apple Shortcuts that require no third-party App Store applications:

### Shortcut 1: Automatic Pairing Shortcut ("Pair ISBN Bridge")
*Used once when starting a scanning session, changing Wi-Fi networks, or after token rotation.*

1. **Receive Input**: Receives JSON config payload (`{"url":"...","token":"..."}`) passed directly from the browser or QR scanner.
2. **Save File**: Writes the configuration dictionary directly to `Shortcuts/isbn_bridge_config.json` in iCloud Drive / On My iPhone (Overwrite: `true`).
3. **Notification**: Vibrates device and shows confirmation ("Paired with PC at [url]").
4. **Auto-Launch Scanner (Optional & Recommended)**: Calls action **"Run Shortcut"** targeting your Scanner Shortcut with *"Wait Until Finished"* disabled, followed by **"Exit Shortcut"**. This creates an end-to-end flow: pointing the camera at the PC immediately transitions straight into book scanning!

> **Zero-Touch Camera Pairing**: Point your normal **iPhone Camera** at the centered QR code on your PC. Tap the yellow web link $\rightarrow$ Safari opens the pairing page and automatically launches the shortcut $\rightarrow$ tap **Open in Shortcuts** $\rightarrow$ configuration is saved and the book scanner launches immediately! No IP lookup or typing required.

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

*(Official iCloud shortcut link will be added here)*

---

## 2. Setting Up the iPhone

1. **Network Connection**:
   - Ensure the iPhone is connected to the same local Wi-Fi network as your PC.
2. **One-Tap Pairing**:
   - Start the Go server and AutoHotkey client on your PC.
   - Point your iPhone's camera at the centered QR code on your monitor.
   - Tap the link, then tap **Open in Shortcuts** to save the configuration automatically.
3. **Scan Books**:
   - Run the **Scanner Shortcut**. The camera scanner opens in a loop. Point it at any book barcode; the ISBN will be validated, authenticated, and pasted onto your PC instantly.

---

## 3. Future Android Support Roadmap

We are designing Android compatibility using the same cryptographic protocol:

- **Protocol Compatibility**: Any client that can generate `SHA-256(ISBN|Timestamp|Token)` and send an HTTP POST request to port `8765` will work seamlessly with the existing Go backend and AutoHotkey client without modifications.
- **Recommended Approaches**:
  1. **HTTP Shortcuts App (F-Droid / Play Store)**: Free, open-source automation app supporting camera barcode triggers, JavaScript hashing (`crypto.subtle`), and HTTP requests.
  2. **Tasker / Automate Script**: Android automation tasks reading stored tokens and sending authenticated POST requests.
  3. **Lightweight PWA / Native APK**: A dedicated, minimal web app running in mobile Chrome using the `BarcodeDetector` API and Web Crypto API.
