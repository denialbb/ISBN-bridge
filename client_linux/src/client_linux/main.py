"""Entry point for ISBN Bridge Linux Client."""

import argparse
import json
import os
import signal
import sys
import threading
import time
import urllib.request
import urllib.error
from typing import Optional

from client_linux.config import AppConfig
from client_linux.i18n import I18n
from client_linux.logger import Logger
from client_linux.paste import PasteEngine
from client_linux.server import HttpListener
from client_linux.server_manager import ServerManager
from client_linux.tooltip import Notifier
from client_linux.tray import TrayManager
from client_linux.ui_qr import QRModal


def send_local_command(
    path: str,
    method: str = "POST",
    body: str = "",
    silent: bool = False,
    conf_path: Optional[str] = None,
) -> bool:
    """Send command to running local client HTTP server."""
    config = AppConfig(conf_path)
    url = f"http://127.0.0.1:{config.http_port}{path}"
    secret_path = HttpListener.find_secret_file(config)
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
            if not silent:
                print(content)
            return True
    except urllib.error.HTTPError as e:
        if not silent:
            print(f"Error {e.code}: {e.read().decode('utf-8', errors='replace')}", file=sys.stderr)
        return False
    except Exception as e:
        if not silent:
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

    # 1. Config & i18n
    config = AppConfig(args.conf)
    I18n.init(config.language)

    # CLI subcommands that communicate with existing running client
    if args.trigger:
        sys.exit(0 if send_local_command("/trigger", method="POST", conf_path=args.conf) else 1)
    if args.show_qr:
        ServerManager.ensure_running(config)
        sys.exit(0 if send_local_command("/qr/show", method="POST", conf_path=args.conf) else 1)
    if args.hide_qr:
        sys.exit(0 if send_local_command("/qr/hide", method="POST", conf_path=args.conf) else 1)
    if args.status:
        sys.exit(0 if send_local_command("/status", method="GET", conf_path=args.conf) else 1)

    # Single-instance application handling:
    # If another instance is already running (e.g. systemd user service),
    # request it to display the pairing QR code and exit cleanly without error.
    if send_local_command("/status", method="GET", silent=True, conf_path=args.conf):
        Logger.log("ISBN Bridge is already running. Requesting existing instance to show QR modal.")
        ServerManager.ensure_running(config)
        send_local_command("/qr/show", method="POST", silent=True, conf_path=args.conf)
        sys.exit(0)

    Logger.log("==================================================")
    Logger.log("             ISBN BRIDGE LINUX CLIENT             ")
    Logger.log("==================================================")
    Logger.log(f"Configuration loaded from {config.file_path}")
    Logger.log(f"Display mode: {'Wayland' if os.environ.get('WAYLAND_DISPLAY') else 'X11'}")

    # 2. Paste Engine
    paste_engine = PasteEngine(config)

    # 3. Local HTTP Listener (:8766)
    listener = HttpListener(config, paste_engine)
    if not listener.start():
        if send_local_command("/status", method="GET", silent=True, conf_path=args.conf):
            Logger.log("ISBN Bridge started by another process. Showing QR modal.")
            ServerManager.ensure_running(config)
            send_local_command("/qr/show", method="POST", silent=True, conf_path=args.conf)
            sys.exit(0)

        msg = I18n.get("listener_error_msg", config.http_port)
        title = I18n.get("listener_error_title")
        Logger.log(f"Fatal: {title} - {msg}")
        Notifier.notify(msg, title, urgency="critical")
        sys.exit(1)

    # 4. Start or attach Go server
    ServerManager.start_or_attach(config)

    stop_event = threading.Event()

    # Shutdown orchestration: request_shutdown() is safe from any thread
    # (tray menu callback, signal handler) — it only signals. The ordered
    # teardown below runs in the main thread, so the interpreter never
    # finalizes while the GTK loop thread is still alive (that segfaults
    # in Py_Exit and systemd restarts us on the resulting core dump).
    def request_shutdown(*_):
        stop_event.set()

    signal.signal(signal.SIGINT, request_shutdown)
    signal.signal(signal.SIGTERM, request_shutdown)

    # 6. Tray Manager
    tray = TrayManager(config, paste_engine, on_exit=request_shutdown)
    tray.start()

    # 7. Notify active
    Notifier.notify(I18n.get("client_active_tip", config.http_port), I18n.get("app_title"))

    # 8. Startup QR modal if configured
    if config.qr_auto_show_on_refresh:
        threading.Thread(target=lambda: (time.sleep(0.4), QRModal.show(config)), daemon=True).start()

    # 9. Main thread wait loop
    try:
        while not stop_event.is_set():
            stop_event.wait(1.0)
    except KeyboardInterrupt:
        pass

    # 10. Ordered teardown in the main thread -> exit code 0, no restart.
    # Each step is guarded: a teardown exception must never turn into a
    # non-zero exit (systemd would restart us on it).
    Logger.log("Shutting down ISBN Bridge Linux client...")
    for step in (
        QRModal.hide,
        paste_engine.shutdown,
        listener.shutdown,
        lambda: ServerManager.shutdown(config),
    ):
        try:
            step()
        except Exception as e:
            Logger.log(f"Shutdown step failed: {e}")
    time.sleep(0.2)  # let the hide land on the GTK loop before quitting it
    try:
        tray.stop()
    except Exception:
        pass
    try:
        tray.join(timeout=5.0)
    except Exception:
        pass


if __name__ == "__main__":
    main()
