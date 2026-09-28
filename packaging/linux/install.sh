#!/usr/bin/env bash
# ISBN Bridge Linux Installer (User-level, no root required)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/isbn-bridge"
APPS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICONS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"
SYSTEMD_USER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/isbn-bridge"

echo "Installing ISBN Bridge to $HOME/.local..."

mkdir -p "$BIN_DIR" "$DATA_DIR" "$APPS_DIR" "$CONFIG_DIR"
mkdir -p "$ICONS_DIR/256x256/apps" "$ICONS_DIR/symbolic/apps" "$SYSTEMD_USER_DIR"

# 1. Install Go server binary
if [[ -f "$SCRIPT_DIR/bin/isbn-bridge-server" ]]; then
    install -m 755 "$SCRIPT_DIR/bin/isbn-bridge-server" "$BIN_DIR/isbn-bridge-server"
elif [[ -f "$SCRIPT_DIR/isbn-bridge-server" ]]; then
    install -m 755 "$SCRIPT_DIR/isbn-bridge-server" "$BIN_DIR/isbn-bridge-server"
else
    echo "Error: isbn-bridge-server binary not found in release package." >&2
    exit 1
fi

# 2. Setup isolated Python venv with system site-packages for PyGObject
echo "Configuring Python environment in $DATA_DIR/venv..."
if command -v uv >/dev/null 2>&1; then
    uv venv "$DATA_DIR/venv" --allow-existing --python /usr/bin/python3 --system-site-packages --quiet
    WHEEL=$(ls -1 "$SCRIPT_DIR"/dist/*.whl 2>/dev/null | head -n 1 || true)
    if [[ -n "$WHEEL" ]]; then
        uv pip install --python "$DATA_DIR/venv/bin/python" "$WHEEL" --quiet
    fi
else
    python3 -m venv "$DATA_DIR/venv" --system-site-packages
    WHEEL=$(ls -1 "$SCRIPT_DIR"/dist/*.whl 2>/dev/null | head -n 1 || true)
    if [[ -n "$WHEEL" ]]; then
        "$DATA_DIR/venv/bin/pip" install "$WHEEL" --quiet
    fi
fi

# 3. Install runner script (remove legacy non-descriptive name if present)
rm -f "$BIN_DIR/isbn-bridge"
install -m 755 "$SCRIPT_DIR/isbn-bridge-client" "$BIN_DIR/isbn-bridge-client"

# 4. Install icons
if [[ -f "$SCRIPT_DIR/assets/tray.png" ]]; then
    install -m 644 "$SCRIPT_DIR/assets/tray.png" "$ICONS_DIR/256x256/apps/isbn-bridge.png"
fi
if [[ -f "$SCRIPT_DIR/assets/isbn-bridge-symbolic.png" ]]; then
    install -m 644 "$SCRIPT_DIR/assets/isbn-bridge-symbolic.png" "$ICONS_DIR/symbolic/apps/isbn-bridge-symbolic.png"
fi

# 5. Install desktop entry
if [[ -f "$SCRIPT_DIR/isbn-bridge.desktop" ]]; then
    install -m 644 "$SCRIPT_DIR/isbn-bridge.desktop" "$APPS_DIR/isbn-bridge.desktop"
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database "$APPS_DIR" 2>/dev/null || true
    fi
fi

# 6. Install systemd user unit
if [[ -f "$SCRIPT_DIR/isbn-bridge.service" ]]; then
    install -m 644 "$SCRIPT_DIR/isbn-bridge.service" "$SYSTEMD_USER_DIR/isbn-bridge.service"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl --user daemon-reload 2>/dev/null || true
    fi
fi

# 7. Default config
if [[ ! -f "$CONFIG_DIR/scanner.conf" && -f "$SCRIPT_DIR/scanner.conf" ]]; then
    install -m 644 "$SCRIPT_DIR/scanner.conf" "$CONFIG_DIR/scanner.conf"
fi

echo ""
echo "Installation complete!"
echo "- Run directly:        $BIN_DIR/isbn-bridge-client"
echo "- Enable at login:     systemctl --user enable --now isbn-bridge.service"
echo "- Desktop application: Available in your application launcher (ISBN Bridge)"
