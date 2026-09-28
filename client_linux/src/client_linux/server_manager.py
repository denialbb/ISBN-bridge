"""Manager for launching, health-checking, and shutting down the Go backend server."""

import os
from pathlib import Path
import signal
import subprocess
import time
import urllib.request
from typing import Optional
from client_linux.config import AppConfig
from client_linux.logger import Logger


class ServerManager:
    server_process: Optional[subprocess.Popen] = None
    is_managed: bool = False

    @classmethod
    def find_binary(cls) -> Optional[Path]:
        repo_root = Path(__file__).resolve().parent.parent.parent.parent
        candidates = [
            Path("bin/isbn-bridge-server"),
            repo_root / "bin" / "isbn-bridge-server",
            repo_root / "isbn-bridge-server",
            Path.cwd() / "bin" / "isbn-bridge-server",
            Path.cwd() / "isbn-bridge-server",
        ]
        for c in candidates:
            if c.is_file() and os.access(c, os.X_OK):
                return c.resolve()
        return None

    @classmethod
    def is_running(cls, config: AppConfig) -> bool:
        try:
            req = urllib.request.Request(f"{config.go_server_url}/health")
            with urllib.request.urlopen(req, timeout=1.0) as resp:
                return resp.status == 200
        except Exception:
            return False

    @classmethod
    def start_or_attach(cls, config: AppConfig) -> bool:
        if cls.is_running(config):
            Logger.log("Go server is already running and healthy")
            return True

        binary = cls.find_binary()
        repo_root = Path(__file__).resolve().parent.parent.parent.parent

        if binary:
            cmd = [str(binary)]
            work_dir = repo_root
        else:
            # Fall back to running via go run if available
            Logger.log("Compiled server binary not found; attempting 'go run ./cmd/server' via mise...")
            cmd = ["go", "run", "./cmd/server"]
            work_dir = repo_root

        try:
            cls.server_process = subprocess.Popen(
                cmd,
                cwd=str(work_dir),
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                preexec_fn=os.setpgrp,
            )
            cls.is_managed = True
            Logger.log(f"Started Go server process (PID: {cls.server_process.pid})")

            # Wait briefly for startup
            for _ in range(25):
                time.sleep(0.2)
                if cls.is_running(config):
                    Logger.log("Go server responded 200 OK to health check")
                    return True

            return cls.is_running(config)
        except Exception as e:
            Logger.log(f"Failed to launch Go server: {e}")
            return False

    @classmethod
    def ensure_running(cls, config: AppConfig) -> bool:
        if cls.is_running(config):
            return True
        return cls.start_or_attach(config)

    @classmethod
    def shutdown(cls, config: AppConfig) -> None:
        if not cls.is_managed and not cls.server_process:
            return

        Logger.log("Shutting down managed Go server...")
        # 1. Request graceful HTTP shutdown
        try:
            req = urllib.request.Request(f"{config.go_server_url}/shutdown", method="POST")
            with urllib.request.urlopen(req, timeout=1.0):
                pass
        except Exception:
            pass

        time.sleep(0.3)

        # 2. Terminate PID if still alive
        if cls.server_process and cls.server_process.poll() is None:
            try:
                os.killpg(os.getpgid(cls.server_process.pid), signal.SIGTERM)
                cls.server_process.wait(timeout=2.0)
            except Exception:
                try:
                    os.killpg(os.getpgid(cls.server_process.pid), signal.SIGKILL)
                except Exception:
                    pass
        cls.server_process = None
        cls.is_managed = False
