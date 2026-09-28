"""Timestamped file logging for ISBN Bridge."""

from datetime import datetime
import os
from pathlib import Path
import subprocess
from typing import Optional


class Logger:
    _log_path: Optional[Path] = None

    @classmethod
    def get_log_path(cls) -> Path:
        if cls._log_path is None:
            # Check if running in git repository
            repo_root = Path(__file__).resolve().parent.parent.parent.parent
            if (repo_root / "go.mod").exists():
                cls._log_path = repo_root / "isbn-bridge-debug.log"
                return cls._log_path

            # Check XDG user config directory
            xdg_config_home = os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config"))
            config_dir = Path(xdg_config_home) / "isbn-bridge"
            if config_dir.exists():
                cls._log_path = config_dir / "isbn-bridge-debug.log"
                return cls._log_path

            cls._log_path = Path.cwd() / "isbn-bridge-debug.log"
        return cls._log_path

    @classmethod
    def set_log_path(cls, path: Path) -> None:
        cls._log_path = path

    @classmethod
    def log(cls, message: str) -> None:
        path = cls.get_log_path()
        timestamp = datetime.now().strftime("%H:%M:%S")
        line = f"{timestamp} | {message}\n"
        try:
            with open(path, "a", encoding="utf-8") as f:
                f.write(line)
        except OSError:
            pass

    @classmethod
    def open_log(cls) -> None:
        path = cls.get_log_path()
        if not path.exists():
            path.touch()
        # Open in default text viewer or editor on Linux
        try:
            subprocess.Popen(["xdg-open", str(path)])
        except OSError:
            pass
