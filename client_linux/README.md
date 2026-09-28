# ISBN Bridge Linux Desktop Client

Linux desktop client for [ISBN Bridge](../README.md), supporting **Wayland (Hyprland, Sway, wlroots)** and **X11**.

---

## Features

- **ISBN Validation**: Mathematical checksum verification for ISBN-10 and ISBN-13 before injecting keystrokes.
- **Hover Paste**:
  - **Wayland (Hyprland)**: Queries `hyprctl cursorpos` and matches window bounding boxes (`hyprctl clients -j`) against `target_tab_title`.
  - **X11**: Queries window under pointer and cursor attributes.
- **Click-to-Paste & Window Focus Trigger**:
  - Automatically executes deferred paste as soon as the target window is focused.
  - Exposes `POST /trigger` / `client-linux --trigger` for global hotkey mapping (e.g., in `hyprland.conf`).
- **Centered QR Pairing Modal**:
  - Borderless, centered Tkinter modal displaying brand logo and pairing QR code from the local Go backend.
- **Audio Tap Feedback**:
  - Plays tap sound using `pw-play`, `paplay`, `aplay`, or `mpv`.
- **Desktop Notifications**:
  - Follow-up visual feedback using `notify-send`.
- **System Tray**:
  - Integrates with `AyatanaAppIndicator3` / D-Bus StatusNotifierItem.

---

## Requirements

- Python 3.12+ managed by **[uv](https://github.com/astral-sh/uv)**
- Wayland / X11 tools:
  - Wayland: `wtype`, `wl-clipboard` (`wl-copy`), `hyprctl` (for Hyprland)
  - X11: `xdotool`, `xclip`
  - Audio: `pipewire` / `pulseaudio` (`pw-play` / `paplay`) or `alsa-utils` (`aplay`)
  - Notifications: `libnotify` (`notify-send`)

---

## Quick Start

### 1. Run the Linux Client

```bash
# Run client directly with uv
cd client_linux && uv run client-linux

# Or from repository root via Makefile
make run-client
```

### 2. Run Tests

```bash
cd client_linux && uv run pytest
# Or
make test-client
```

---

## Hyprland Integration

To bind an explicit hotkey to paste the currently armed ISBN:

Add to `~/.config/hypr/hyprland.conf`:

```ini
# Paste armed ISBN into currently focused window
bind = $mainMod, V, exec, uv --directory /home/denial/Projects/ISBN-bridge/client_linux run client-linux --trigger
```

---

## CLI Options

```bash
uv run client-linux --trigger       # Triggers immediate deferred paste
uv run client-linux --show-qr       # Shows the centered pairing QR modal
uv run client-linux --hide-qr       # Hides the pairing QR modal
uv run client-linux --reset-token   # Rotates pairing token immediately
uv run client-linux --status        # Checks client status (JSON)
uv run client-linux --conf <path>   # Custom scanner.conf path
```
