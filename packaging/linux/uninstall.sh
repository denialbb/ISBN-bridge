#!/usr/bin/env bash
# ISBN Bridge Linux Uninstaller
set -e

BIN_DIR="${XDG_BIN_HOME:-$HOME/.local/bin}"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/isbn-bridge"
APPS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICONS_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor"
SYSTEMD_USER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

echo "Uninstalling ISBN Bridge..."

if command -v systemctl >/dev/null 2>&1; then
    systemctl --user stop isbn-bridge.service 2>/dev/null || true
    systemctl --user disable isbn-bridge.service 2>/dev/null || true
fi

rm -f "$BIN_DIR/isbn-bridge-server"
rm -f "$BIN_DIR/isbn-bridge-client"
rm -f "$BIN_DIR/isbn-bridge"
rm -rf "$DATA_DIR"
rm -f "$APPS_DIR/isbn-bridge.desktop"
rm -f "$ICONS_DIR/256x256/apps/isbn-bridge.png"
rm -f "$ICONS_DIR/symbolic/apps/isbn-bridge-symbolic.png"
rm -f "$SYSTEMD_USER_DIR/isbn-bridge.service"

if command -v systemctl >/dev/null 2>&1; then
    systemctl --user daemon-reload 2>/dev/null || true
fi
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$APPS_DIR" 2>/dev/null || true
fi

echo "ISBN Bridge uninstalled successfully."
