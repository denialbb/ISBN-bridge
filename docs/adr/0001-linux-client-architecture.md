# ADR 0001: Linux Client Substitution Layer Architecture

## Status
Accepted

## Context
The existing desktop client is implemented in AutoHotkey v2 (`client/main.ahk`), tying desktop interaction to Windows Win32 APIs. On Linux, display servers are split across Wayland (Hyprland, Sway, GNOME, KDE) and X11. Wayland isolates client windows, preventing arbitrary background applications from reading cursor shape (`IBeam`) or sniffing global mouse clicks without compositor cooperation.

## Decision
1. **Implementation Stack**: Implement a modular Python 3 client located in `client_linux/`, managed via `uv` with its own `pyproject.toml`.
2. **Display Server Abstraction**:
   - **Wayland (Hyprland)**: Use `hyprctl cursorpos` and `hyprctl clients -j` to detect hovered windows, `hyprctl activewindow -j` for focused window validation, and `wtype` (virtual-keyboard protocol) / `wl-copy` for keystroke and clipboard injection.
   - **X11**: Use Xlib / `xdotool` for window targeting, XFixes for IBeam cursor shape verification, and `xclip` for clipboard manipulation.
3. **Deferred Paste & Click Triggering on Wayland**:
   - Provide dual trigger mechanisms:
     a) Automatic deferred paste upon target window focus change (monitored via Hyprland event socket / polling).
     b) HTTP/CLI trigger endpoint (`POST /trigger` or `isbn-bridge-client --trigger`) allowing users to bind any custom key/mouse combination in `hyprland.conf`.
4. **Security & Validation**:
   - Strictly validate ISBN-10 and ISBN-13 checksums within the client layer before initiating any input injection.
   - Enforce shared-secret verification via `X-ISBN-Bridge-Local` from `local_secret.txt`.
5. **UI & Media**:
   - Use Tkinter with Pillow for the centered QR pairing modal and floating overlays.
   - Use desktop notification protocols (`notify-send`) and native audio commands (`pw-play`, `paplay`, `aplay`).
   - Support system tray via `AyatanaAppIndicator3` / D-Bus StatusNotifierItem.

## Consequences
- Full parity with Windows AutoHotkey client across all features.
- Seamless execution under Wayland (Hyprland) without requiring root or membership in `/dev/input` group.
- Coexists cleanly with the Windows client in the repository.
