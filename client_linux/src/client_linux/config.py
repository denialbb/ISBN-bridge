"""Configuration loader and writer for scanner.conf."""

from configparser import ConfigParser
import os
from pathlib import Path
from typing import Optional


class AppConfig:
    def __init__(self, file_path: Optional[str] = None):
        self.file_path = file_path or self.find_config_file()
        self.http_port: int = 8766
        self.go_port: int = 8765
        self.go_server_url: str = "http://127.0.0.1:8765"
        self.token_ttl_minutes: int = 60
        self.hide_console: bool = True

        self.target_tab_title: str = "hardcover"
        self.overwrite_existing_text: bool = True
        self.auto_paste_on_hover: bool = True
        self.play_tap_sound: bool = True
        self.sound_file: str = "sounds/tap.wav"
        self.tooltip_offset_x: int = 10
        self.tooltip_offset_y: int = 12

        self.qr_auto_show_on_refresh: bool = True
        self.qr_auto_hide_seconds: int = 45
        self.qr_popup_size: int = 280
        self.language: str = "auto"

        self.load()

    @staticmethod
    def find_config_file() -> str:
        candidates = [
            Path("scanner.conf"),
            Path(__file__).resolve().parent.parent.parent.parent / "scanner.conf",
            Path.cwd() / "scanner.conf",
            Path.cwd().parent / "scanner.conf",
        ]
        for c in candidates:
            if c.is_file():
                return str(c.resolve())
        return "scanner.conf"

    def load(self) -> None:
        if not os.path.isfile(self.file_path):
            return

        parser = ConfigParser()
        parser.read(self.file_path, encoding="utf-8")

        if parser.has_section("Server"):
            self.http_port = parser.getint("Server", "ahk_port", fallback=self.http_port)
            self.go_port = parser.getint("Server", "port", fallback=self.go_port)
            self.go_server_url = f"http://127.0.0.1:{self.go_port}"
            self.token_ttl_minutes = parser.getint("Server", "token_ttl_minutes", fallback=self.token_ttl_minutes)
            self.hide_console = parser.getboolean("Server", "hide_console", fallback=self.hide_console)

        if parser.has_section("AutoPaste"):
            self.target_tab_title = parser.get("AutoPaste", "target_tab_title", fallback=self.target_tab_title).strip()
            self.overwrite_existing_text = parser.getboolean("AutoPaste", "overwrite_existing_text", fallback=self.overwrite_existing_text)
            self.auto_paste_on_hover = parser.getboolean("AutoPaste", "auto_paste_on_hover", fallback=self.auto_paste_on_hover)
            self.play_tap_sound = parser.getboolean("AutoPaste", "play_tap_sound", fallback=self.play_tap_sound)
            self.sound_file = parser.get("AutoPaste", "sound_file", fallback=self.sound_file).strip()
            self.tooltip_offset_x = parser.getint("AutoPaste", "tooltip_offset_x", fallback=self.tooltip_offset_x)
            self.tooltip_offset_y = parser.getint("AutoPaste", "tooltip_offset_y", fallback=self.tooltip_offset_y)

        if parser.has_section("QRCode"):
            self.qr_auto_show_on_refresh = parser.getboolean("QRCode", "auto_show_on_refresh", fallback=self.qr_auto_show_on_refresh)
            self.qr_auto_hide_seconds = parser.getint("QRCode", "auto_hide_seconds", fallback=self.qr_auto_hide_seconds)
            self.qr_popup_size = parser.getint("QRCode", "popup_size", fallback=self.qr_popup_size)

        if parser.has_section("UI"):
            self.language = parser.get("UI", "language", fallback=self.language).strip()

    def save(self, section: str, key: str, value: str) -> None:
        if not os.path.isfile(self.file_path):
            return

        parser = ConfigParser()
        parser.read(self.file_path, encoding="utf-8")

        if not parser.has_section(section):
            parser.add_section(section)

        parser.set(section, key, str(value))
        with open(self.file_path, "w", encoding="utf-8") as f:
            parser.write(f)
