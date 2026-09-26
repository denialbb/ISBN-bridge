# ISBN Bridge on Android (HTTP Shortcuts)

Same workflow as iOS, no code to maintain: scan a book barcode, sign it
with SHA-256, POST it to your PC, loop.

## 1. Install

- [HTTP Shortcuts](https://f-droid.org/packages/ch.rmy.android.http_shortcuts/)
  (free, open source — also on the Play Store)
- [Binary Eye](https://f-droid.org/packages/de.markusfisch.android.binaryeye/)
  (free barcode scanner backend used by the script)

## 2. Pair (once per token rotation)

1. On the phone, open the pairing link shown by the PC QR popup
   (`http://<pc-ip>:8765/pair`).
2. Copy the `url` and `token` values into two global variables:
   - `isbn_server_url` → the `url` (e.g. `http://192.168.1.50:8765`)
   - `isbn_token` → the `token` (tick *Treat value as secret*)

## 3. Create the shortcut

New shortcut named `ISBN Bridge Scanner`:

- **Method / URL**: `POST`, URL `{isbn_server_url}/isbn`
- **Body**: request body type *Custom Text*, content `{isbn_current}`,
  content type `text/plain`
- **Headers**:
  - `Authorization`: `Bearer {isbn_sig}`
  - `Timestamp`: `{isbn_ts}`
- **Scripting → Run before execution**: paste `scanner.js` from this folder
- Optional: home-screen widget / Quick Settings tile for one-tap start

## 4. Scan

Run the shortcut and point the camera at book barcodes. The phone
vibrates per scan; cancel the scanner to stop the loop. Server responses:

- `200 OK` — pasted on the PC
- `401` — bad signature or stale timestamp (re-pair if the token rotated)
- `409` — duplicate (same scan submitted twice)
- `429` — scanning faster than 2 per 10 s; slow down slightly

## Verification

`scanner.js` was checked with `node --check` and against the Go server:
valid scans authenticate, tampered signatures get `401`, replays get `409`.

Binary Eye handoff on real hardware is not yet tested — if you try it,
please open an issue with your phone model and the result.
