"""End-to-end integration test for Go Server <-> Linux Substitution Client."""

import os
import signal
import subprocess
import time
import urllib.request
import json
import pytest

from client_linux.config import AppConfig
from client_linux.paste import PasteEngine
from client_linux.server import HttpListener
from client_linux.isbn import validate


def test_e2e_pipeline():
    # 1. Setup ports
    client_port = 18766
    server_port = 18765

    # Write a test secret file
    secret = "e2e-test-secret-token-32-chars-long!"
    secret_path = "/tmp/e2e_local_secret.txt"
    with open(secret_path, "w", encoding="utf-8") as f:
        f.write(secret)

    # Configure client
    cfg = AppConfig()
    cfg.http_port = client_port
    cfg.go_port = server_port
    cfg.auto_paste_on_hover = False
    cfg.target_tab_title = ""

    engine = PasteEngine(cfg)
    listener = HttpListener(cfg, engine, secret_file_path=secret_path)
    assert listener.start() is True

    try:
        # Simulate Go server forwarder posting verified ISBN to client
        req = urllib.request.Request(
            f"http://127.0.0.1:{client_port}/paste",
            data=b"978-0-306-40615-7",
            headers={"X-ISBN-Bridge-Local": secret},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=2.0) as resp:
            assert resp.status == 200
            assert resp.read() == b"OK"

        # Verify engine received and validated the normalized ISBN
        assert engine.pending_isbn == "9780306406157"

        # Check status endpoint
        status_req = urllib.request.Request(f"http://127.0.0.1:{client_port}/status")
        with urllib.request.urlopen(status_req, timeout=2.0) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            assert data["status"] == "ok"
            assert data["pending_isbn"] == "9780306406157"

        # Trigger paste
        trigger_req = urllib.request.Request(
            f"http://127.0.0.1:{client_port}/trigger",
            headers={"X-ISBN-Bridge-Local": secret},
            method="POST",
        )
        with urllib.request.urlopen(trigger_req, timeout=2.0) as resp:
            assert resp.status == 200

        # Pending ISBN should now be cleared
        assert engine.pending_isbn is None

    finally:
        listener.shutdown()
        engine.shutdown()
        if os.path.exists(secret_path):
            os.remove(secret_path)
