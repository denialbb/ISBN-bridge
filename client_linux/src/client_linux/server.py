"""Local HTTP server listening on 127.0.0.1 for commands from the Go forwarder."""

from http.server import HTTPServer, BaseHTTPRequestHandler
import hmac
import json
import os
from pathlib import Path
import threading
from typing import Optional

from client_linux.config import AppConfig
from client_linux.isbn import validate, ISBNValidationError
from client_linux.logger import Logger
from client_linux.paste import PasteEngine
from client_linux.ui_qr import QRModal


class BridgeRequestHandler(BaseHTTPRequestHandler):
    server: "HttpListenerServer"

    def log_message(self, format: str, *args) -> None:
        # Route access logs to our Logger instead of stderr
        Logger.log(f"HTTP {self.command} {self.path} - {format % args}")

    def send_plain_response(self, status_code: int, message: str) -> None:
        data = message.encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(data)

    def send_json_response(self, status_code: int, payload: dict) -> None:
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(data)

    def check_local_secret(self) -> bool:
        secret_file = self.server.secret_file_path or self.server.find_secret_file()
        if not secret_file or not os.path.isfile(secret_file):
            if not self.server.secret_warned:
                self.server.secret_warned = True
                Logger.log("Warning: local_secret.txt not found; accepting localhost requests without forwarder secret")
            return True

        try:
            with open(secret_file, "r", encoding="utf-8") as f:
                expected = f.read().strip()
        except OSError:
            return False

        if not expected:
            return False

        received = self.headers.get("X-ISBN-Bridge-Local", "").strip()
        return hmac.compare_digest(received, expected)

    def do_POST(self) -> None:
        if not self.check_local_secret():
            Logger.log("Rejected request: missing or invalid forwarder secret")
            self.send_plain_response(403, "Missing or invalid forwarder secret")
            return

        try:
            content_length = int(self.headers.get("Content-Length", 0))
            if content_length < 0 or content_length > 16384:
                self.send_plain_response(400 if content_length < 0 else 413, "Invalid Content-Length")
                return
        except (ValueError, TypeError):
            self.send_plain_response(400, "Malformed Content-Length")
            return

        body = self.rfile.read(content_length).decode("utf-8", errors="replace").strip()

        path = self.path.split("?")[0]

        if path in ("/paste", "/isbn"):
            if not body:
                self.send_plain_response(400, "Missing ISBN")
                return

            try:
                # Validates ISBN mathematically
                self.server.paste_engine.arm(body)
                self.send_plain_response(200, "OK")
            except ISBNValidationError as e:
                Logger.log(f"Rejected invalid ISBN '{body}': {e}")
                self.send_plain_response(400, f"Invalid ISBN: {e}")
            except Exception as e:
                Logger.log(f"Error processing ISBN: {e}")
                self.send_plain_response(500, str(e))
            return

        elif path == "/qr/show":
            try:
                QRModal.show(self.server.config)
                self.send_plain_response(200, "QR shown")
            except Exception as e:
                Logger.log(f"Error showing QR modal: {e}")
                self.send_plain_response(500, str(e))
            return

        elif path == "/qr/hide":
            try:
                QRModal.hide()
                self.send_plain_response(200, "QR hidden")
            except Exception as e:
                Logger.log(f"Error hiding QR modal: {e}")
                self.send_plain_response(500, str(e))
            return

        elif path in ("/trigger", "/paste-now"):
            success = self.server.paste_engine.trigger()
            if success:
                self.send_plain_response(200, "Triggered")
            else:
                self.send_plain_response(200, "No pending ISBN")
            return

        else:
            self.send_plain_response(404, "Not Found")

    def do_GET(self) -> None:
        path = self.path.split("?")[0]
        if path in ("/health", "/status"):
            self.send_json_response(200, {
                "status": "ok",
                "enabled": self.server.paste_engine.enabled,
                "pending_isbn": self.server.paste_engine.pending_isbn,
                "target": self.server.config.target_tab_title,
            })
            return
        elif path == "/trigger":
            if not self.check_local_secret():
                self.send_plain_response(403, "Missing forwarder secret")
                return
            success = self.server.paste_engine.trigger()
            self.send_plain_response(200, "Triggered" if success else "No pending ISBN")
            return

        self.send_plain_response(404, "Not Found")


class HttpListenerServer(HTTPServer):
    def __init__(self, server_address, RequestHandlerClass, config: AppConfig, paste_engine: PasteEngine, secret_file_path: Optional[str] = None):
        super().__init__(server_address, RequestHandlerClass)
        self.config = config
        self.paste_engine = paste_engine
        self.secret_file_path = secret_file_path or self.find_secret_file()
        self.secret_warned = False

    @staticmethod
    def find_secret_file() -> Optional[str]:
        candidates = [
            Path("local_secret.txt"),
            Path(__file__).resolve().parent.parent.parent.parent / "local_secret.txt",
            Path.cwd() / "local_secret.txt",
            Path.cwd().parent / "local_secret.txt",
        ]
        for c in candidates:
            if c.is_file():
                return str(c.resolve())
        return None


class HttpListener:
    def __init__(self, config: AppConfig, paste_engine: PasteEngine, secret_file_path: Optional[str] = None):
        self.config = config
        self.paste_engine = paste_engine
        self.secret_file_path = secret_file_path
        self.server: Optional[HttpListenerServer] = None
        self.thread: Optional[threading.Thread] = None

    def start(self) -> bool:
        port = self.config.http_port
        try:
            self.server = HttpListenerServer(
                ("127.0.0.1", port),
                BridgeRequestHandler,
                self.config,
                self.paste_engine,
                self.secret_file_path,
            )
            self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
            self.thread.start()
            Logger.log(f"HTTP listener active on 127.0.0.1:{port}")
            return True
        except Exception as e:
            Logger.log(f"Failed to start HTTP listener on port {port}: {e}")
            return False

    def shutdown(self) -> None:
        if self.server:
            try:
                self.server.shutdown()
                self.server.server_close()
            except Exception:
                pass
            self.server = None
        Logger.log("HTTP listener stopped")
