"""Notification and tooltip feedback for Linux desktop."""

import shutil
import subprocess
from typing import Optional
from client_linux.i18n import I18n
from client_linux.logger import Logger


class Notifier:
    @staticmethod
    def notify(message: str, title: Optional[str] = None, timeout_ms: int = 3000, urgency: str = "normal") -> None:
        title = title or I18n.get("app_title")
        Logger.log(f"Notification: [{title}] {message}")

        import os
        if os.getenv("PYTEST_CURRENT_TEST") or os.getenv("CI") or os.getenv("ISBN_BRIDGE_NO_NOTIFY"):
            return

        notify_cmd = shutil.which("notify-send")
        if notify_cmd:
            try:
                subprocess.Popen(
                    [
                        notify_cmd,
                        "-a", "ISBN Bridge",
                        "-u", urgency,
                        "-t", str(timeout_ms),
                        title,
                        message,
                    ],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                )
            except Exception as e:
                Logger.log(f"notify-send failed: {e}")


class FollowToolTip:
    @staticmethod
    def show(text: str) -> None:
        Notifier.notify(text, timeout_ms=3000)

    @staticmethod
    def flash(text: str, duration_ms: int = 1500) -> None:
        Notifier.notify(text, timeout_ms=duration_ms)

    @staticmethod
    def hide() -> None:
        pass
