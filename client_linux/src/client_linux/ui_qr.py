"""Centered borderless QR modal popup for desktop pairing."""

import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import time
import tkinter as tk
import urllib.request
from typing import Optional
from PIL import Image, ImageTk

from client_linux.config import AppConfig
from client_linux.i18n import I18n
from client_linux.logger import Logger


class QRModal:
    _window: Optional[tk.Toplevel] = None
    _root: Optional[tk.Tk] = None
    _auto_hide_timer_id: Optional[str] = None
    _brand_photo: Optional[ImageTk.PhotoImage] = None
    _qr_photo: Optional[ImageTk.PhotoImage] = None
    _lock = threading.Lock()

    @classmethod
    def set_root(cls, root: tk.Tk) -> None:
        cls._root = root

    @classmethod
    def get_root(cls) -> tk.Tk:
        if cls._root is None:
            cls._root = tk.Tk()
            cls._root.withdraw()
        return cls._root

    @classmethod
    def show(cls, config: AppConfig) -> None:
        """Thread-safe show method."""
        root = cls.get_root()
        root.after(0, lambda: cls._show_internal(config))

    @classmethod
    def hide(cls) -> None:
        """Thread-safe hide method."""
        root = cls.get_root()
        root.after(0, cls._hide_internal)

    @classmethod
    def _fetch_qr_image(cls, server_url: str) -> Optional[Image.Image]:
        url = f"{server_url}/qr.png"
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "ISBN-Bridge-Linux/1.0"})
            with urllib.request.urlopen(req, timeout=3.0) as resp:
                data = resp.read()
                return Image.open(io.BytesIO(data))
        except Exception as e:
            Logger.log(f"QRModal: failed to fetch QR from local Go server {url}: {e}")
            return None

    @classmethod
    def _find_brand_asset(cls) -> Optional[Path]:
        repo_root = Path(__file__).resolve().parent.parent.parent.parent
        candidates = [
            repo_root / "client" / "assets" / "brand.png",
            Path.cwd() / "client" / "assets" / "brand.png",
            Path("client/assets/brand.png"),
        ]
        for c in candidates:
            if c.is_file():
                return c.resolve()
        return None

    @classmethod
    def _get_active_screen_geometry(cls, win: tk.Toplevel) -> tuple[int, int, int, int]:
        """Return (offset_x, offset_y, width, height) of current monitor."""
        if os.environ.get("HYPRLAND_INSTANCE_SIGNATURE"):
            try:
                out = subprocess.check_output(["hyprctl", "monitors", "-j"], timeout=0.5)
                monitors = json.loads(out)
                for m in monitors:
                    if m.get("focused"):
                        return (
                            int(m.get("x", 0)),
                            int(m.get("y", 0)),
                            int(m.get("width", 1366)),
                            int(m.get("height", 768)),
                        )
            except Exception:
                pass
        return (0, 0, win.winfo_screenwidth(), win.winfo_screenheight())

    @classmethod
    def _show_internal(cls, config: AppConfig) -> None:
        with cls._lock:
            if cls._window is not None:
                try:
                    if cls._auto_hide_timer_id:
                        cls._window.after_cancel(cls._auto_hide_timer_id)
                        cls._auto_hide_timer_id = None
                    cls._window.destroy()
                except Exception:
                    pass
                cls._window = None

            # Fetch QR directly from local Go backend in memory
            orig_qr = cls._fetch_qr_image(config.go_server_url)
            if not orig_qr:
                Logger.log("QRModal: cannot display QR, local Go server unreachable")
                return

            root = cls.get_root()
            win = tk.Toplevel(root)
            cls._window = win

            # Keep withdrawn until geometry is fully computed to avoid top-left XWayland map bug
            win.withdraw()
            win.overrideredirect(True)
            win.attributes("-topmost", True)
            win.configure(bg="#FFFFFF", padx=16, pady=6)

            size = config.qr_popup_size

            # 1. Brand header
            brand_path = cls._find_brand_asset()
            if brand_path and brand_path.is_file():
                try:
                    orig_brand = Image.open(brand_path)
                    brand_h = max(20, (size * 55) // 516)
                    resized_brand = orig_brand.resize((size, brand_h), Image.Resampling.LANCZOS)
                    cls._brand_photo = ImageTk.PhotoImage(resized_brand)
                    brand_label = tk.Label(win, image=cls._brand_photo, bg="#FFFFFF", cursor="hand2")
                    brand_label.pack(pady=(2, 4))
                    brand_label.bind("<Button-1>", lambda e: cls.hide())
                except Exception as e:
                    Logger.log(f"QRModal: error loading brand image: {e}")

            # 2. QR Image generated by Go backend
            try:
                resized_qr = orig_qr.resize((size, size), Image.Resampling.NEAREST)
                cls._qr_photo = ImageTk.PhotoImage(resized_qr)
                qr_label = tk.Label(win, image=cls._qr_photo, bg="#FFFFFF", cursor="hand2")
                qr_label.pack(pady=2)
                qr_label.bind("<Button-1>", lambda e: cls.hide())
            except Exception as e:
                Logger.log(f"QRModal: error rendering QR image: {e}")
                win.destroy()
                cls._window = None
                return

            # 3. Hint text
            hint_text = I18n.get("qr_hint")
            hint_label = tk.Label(
                win,
                text=hint_text,
                bg="#FFFFFF",
                fg="#2F4A6E",
                font=("sans-serif", 9),
                cursor="hand2",
            )
            hint_label.pack(pady=(6, 4))
            hint_label.bind("<Button-1>", lambda e: cls.hide())

            # Bind dismiss events
            win.bind("<Button-1>", lambda e: cls.hide())
            win.bind("<Escape>", lambda e: cls.hide())

            # Position in exact screen center
            win.update_idletasks()
            win_w = win.winfo_reqwidth()
            win_h = win.winfo_reqheight()
            mon_x, mon_y, screen_w, screen_h = cls._get_active_screen_geometry(win)
            pos_x = mon_x + (screen_w - win_w) // 2
            pos_y = mon_y + (screen_h - win_h) // 2
            win.geometry(f"{win_w}x{win_h}+{pos_x}+{pos_y}")

            # Reveal centered
            win.deiconify()
            win.lift()
            win.focus_force()
            win.update()
            Logger.log(f"QRModal shown at center ({pos_x}, {pos_y})")

            # Auto-hide timer
            if config.qr_auto_hide_seconds > 0:
                cls._auto_hide_timer_id = win.after(
                    config.qr_auto_hide_seconds * 1000, cls.hide
                )

    @classmethod
    def _hide_internal(cls) -> None:
        with cls._lock:
            if cls._window is not None:
                try:
                    if cls._auto_hide_timer_id:
                        cls._window.after_cancel(cls._auto_hide_timer_id)
                        cls._auto_hide_timer_id = None
                    cls._window.destroy()
                except Exception:
                    pass
                finally:
                    cls._window = None
                    Logger.log("QRModal hidden")
