"""Entry point for ISBN Bridge Linux Client."""

import argparse
import json
import os
import signal
import sys
import time
import tkinter as tk
import urllib.request
import urllib.error

from client_linux.config import AppConfig
from client_linux.i18n import I18n
from client_linux.logger import Logger
from client_linux.paste import PasteEngine
from client_linux.server import HttpListener
from client_linux.server_manager import ServerManager
from client_linux.tooltip import Notifier
from client_linux.tray import TrayManager
from client_linux.ui_qr import QRModal


def send_local_command(path: str, method: str = "POST", body: str = "") -> bool:
    """Send command to running local client HTTP server."""
    config = AppConfig()
    url = f"http://127.0.0.1:{config.http_port}{path}"
    secret_path = HttpListener.find_secret_file()
    headers = {}
    if secret_path and os.path.isfile(secret_path):
        try:
            with open(secret_path, "r", encoding="utf-8") as f:
                headers["X-ISBN-Bridge-Local"] = f.read().strip()
        except OSError:
            pass

    try:
        data = body.encode("utf-8") if body else None
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        with urllib.request.urlopen(req, timeout=1.5) as resp:
            content = resp.read().decode("utf-8", errors="replace")
            print(content)
            return True
    except urllib.error.HTTPError as e:
        print(f"Error {e.code}: {e.read().decode('utf-8', errors='replace')}", file=sys.stderr)
        return False
    except Exception as e:
        print(f"Could not reach local ISBN Bridge client at {url}: {e}", file=sys.stderr)
        return False


def main():
    parser = argparse.ArgumentParser(description="ISBN Bridge Linux Desktop Client")
    parser.add_argument("--trigger", action="store_true", help="Trigger immediate paste of pending ISBN")
    parser.add_argument("--show-qr", action="store_true", help="Show the centered pairing QR modal")
    parser.add_argument("--hide-qr", action="store_true", help="Hide the pairing QR modal")
    parser.add_argument("--reset-token", action="store_true", help="Rotate pairing token immediately")
    parser.add_argument("--status", action="store_true", help="Check client status")
    parser.add_argument("--conf", type=str, default=None, help="Path to scanner.conf")
    args = parser.parse_args()

    # CLI subcommands that communicate with existing running client
    if args.trigger:
        sys.exit(0 if send_local_command("/trigger", method="POST") else 1)
    if args.show_qr:
        sys.exit(0 if send_local_command("/qr/show", method="POST") else 1)
    if args.hide_qr:
        sys.exit(0 if send_local_command("/qr/hide", method="POST") else 1)
    if args.status:
        sys.exit(0 if send_local_command("/status", method="GET") else 1)

    Logger.log("==================================================")
    Logger.log("             ISBN BRIDGE LINUX CLIENT             ")
    Logger.log("==================================================")

    # 1. Config & i18n
    config = AppConfig(args.conf)
    I18n.init(config.language)
    Logger.log(f"Configuration loaded from {config.file_path}")
    Logger.log(f"Display mode: {'Wayland' if os.environ.get('WAYLAND_DISPLAY') else 'X11'}")

    # 2. Tkinter Root for Centered QR Modal
    root = tk.Tk()
    root.withdraw()
    QRModal.set_root(root)

    # 3. Paste Engine
    paste_engine = PasteEngine(config)

    # 4. Local HTTP Listener (:8766)
    listener = HttpListener(config, paste_engine)
    if not listener.start():
        msg = I18n.get("listener_error_msg", config.http_port)
        title = I18n.get("listener_error_title")
        Logger.log(f"Fatal: {title} - {msg}")
        Notifier.notify(msg, title, urgency="critical")
        sys.exit(1)

    # 5. Start or attach Go server
    ServerManager.start_or_attach(config)

    # 6. Shutdown handler
    def cleanup(*_):
        Logger.log("Shutting down ISBN Bridge Linux client...")
        try:
            QRModal.hide()
        except Exception:
            pass
        paste_engine.shutdown()
        listener.shutdown()
        ServerManager.shutdown(config)
        try:
            root.destroy()
        except Exception:
            pass
        sys.exit(0)

    signal.signal(signal.SIGINT, cleanup)
    signal.signal(signal.SIGTERM, cleanup)

    # 7. Tray Manager
    tray = TrayManager(config, paste_engine, on_exit=cleanup)
    tray.start()

    # 8. Notify active
    Notifier.notify(I18n.get("client_active_tip", config.http_port), I18n.get("app_title"))

    # 9. Startup QR modal if configured
    if config.qr_auto_show_on_refresh:
        root.after(400, lambda: QRModal.show(config))

    # 10. Run Tkinter event loop in main thread
    try:
        root.mainloop()
    except KeyboardInterrupt:
        cleanup()


if __name__ == "__main__":
    main()
