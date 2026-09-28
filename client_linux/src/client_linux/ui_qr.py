"""Centered borderless QR modal popup for desktop pairing."""

import io
import json
import os
from pathlib import Path
import subprocess
import threading
import time
import urllib.request
from typing import Optional
from PIL import Image

from client_linux.config import AppConfig
from client_linux.i18n import I18n
from client_linux.logger import Logger
from client_linux.server_manager import ServerManager


def _has_gtk() -> bool:
    """Check if GTK3 and GLib are available in runtime."""
    try:
        from client_linux.tray import _ensure_gi
        _ensure_gi()
        import gi
        gi.require_version("Gtk", "3.0")
        from gi.repository import Gtk, Gdk, GdkPixbuf, GLib
        return True
    except Exception:
        return False


def _has_tkinter() -> bool:
    """Check if Tkinter is available in runtime without crashing."""
    try:
        import tkinter as tk
        from PIL import ImageTk
        return True
    except Exception:
        return False


class QRModal:
    _gtk_window = None
    _gtk_timer_id = None

    _tk_window = None
    _tk_root = None
    _tk_timer_id = None
    _brand_photo = None
    _qr_photo = None

    _lock = threading.Lock()

    @classmethod
    def set_root(cls, root) -> None:
        cls._tk_root = root

    @classmethod
    def _get_tk_root(cls):
        if cls._tk_root is None and _has_tkinter():
            try:
                import tkinter as tk
                cls._tk_root = tk.Tk()
                cls._tk_root.withdraw()
            except Exception:
                pass
        return cls._tk_root

    @classmethod
    def show(cls, config: AppConfig) -> None:
        """Thread-safe show method using GTK or Tkinter."""
        if _has_gtk():
            try:
                from gi.repository import GLib
                GLib.idle_add(cls._show_gtk, config)
                return
            except Exception as e:
                Logger.log(f"QRModal: failed to schedule GTK show: {e}")

        if _has_tkinter():
            try:
                root = cls._get_tk_root()
                if root:
                    root.after(0, lambda: cls._show_tk(config))
                    return
            except Exception as e:
                Logger.log(f"QRModal: failed to schedule Tkinter show: {e}")

        Logger.log("QRModal: no GUI toolkit available to display QR")

    @classmethod
    def hide(cls) -> None:
        """Thread-safe hide method."""
        if _has_gtk():
            try:
                from gi.repository import GLib
                GLib.idle_add(cls._hide_gtk)
            except Exception:
                pass

        if _has_tkinter():
            try:
                root = cls._get_tk_root()
                if root:
                    root.after(0, cls._hide_tk)
            except Exception:
                pass

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
        pkg_brand = Path(__file__).resolve().parent / "assets" / "brand.png"
        repo_root = Path(__file__).resolve().parent.parent.parent.parent
        candidates = [
            pkg_brand,
            repo_root / "client" / "assets" / "brand.png",
            Path.cwd() / "client" / "assets" / "brand.png",
            Path("client/assets/brand.png"),
        ]
        for c in candidates:
            if c.is_file():
                return c.resolve()
        return None

    @classmethod
    def _get_active_screen_geometry(cls, win) -> tuple[int, int, int, int]:
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
        try:
            return (0, 0, win.winfo_screenwidth(), win.winfo_screenheight())
        except Exception:
            return (0, 0, 1920, 1080)

    # -------------------------------------------------------------------------
    # Native GTK3 Implementation
    # -------------------------------------------------------------------------

    @classmethod
    def _show_gtk(cls, config: AppConfig) -> bool:
        with cls._lock:
            cls._hide_gtk()

            from gi.repository import Gtk, Gdk, GdkPixbuf, GLib

            # 1. Fetch QR directly from local Go backend in memory (auto-start server if needed)
            orig_qr = cls._fetch_qr_image(config.go_server_url)
            if not orig_qr:
                ServerManager.start_or_attach(config)
                orig_qr = cls._fetch_qr_image(config.go_server_url)
            if not orig_qr:
                Logger.log("QRModal: cannot display QR, local Go server unreachable")
                return False

            size = config.qr_popup_size

            # 2. Compose brand image and QR code using PIL
            brand_path = cls._find_brand_asset()
            brand_img = None
            brand_h = 0
            if brand_path and brand_path.is_file():
                try:
                    orig_brand = Image.open(brand_path)
                    brand_h = max(20, (size * 55) // 516)
                    brand_img = orig_brand.resize((size, brand_h), Image.Resampling.LANCZOS)
                except Exception as e:
                    Logger.log(f"QRModal: error loading brand image: {e}")

            qr_img = orig_qr.resize((size, size), Image.Resampling.NEAREST)

            total_w = size
            spacing = 8 if brand_img else 0
            total_h = (brand_h + spacing + size) if brand_img else size

            canvas = Image.new("RGBA", (total_w, total_h), (255, 255, 255, 255))
            if brand_img:
                mask = brand_img if brand_img.mode == "RGBA" else None
                canvas.paste(brand_img, (0, 0), mask)
            qr_mask = qr_img if qr_img.mode == "RGBA" else None
            canvas.paste(qr_img, (0, brand_h + spacing if brand_img else 0), qr_mask)

            buf = io.BytesIO()
            canvas.save(buf, format="PNG")
            png_bytes = buf.getvalue()

            # 3. Create GTK window
            win = Gtk.Window(type=Gtk.WindowType.TOPLEVEL)
            win.set_title("ISBN Bridge - Pairing QR")
            win.set_decorated(False)
            win.set_position(Gtk.WindowPosition.CENTER)
            win.set_keep_above(True)
            win.set_skip_taskbar_hint(True)
            win.set_skip_pager_hint(True)
            win.set_type_hint(Gdk.WindowTypeHint.DIALOG)

            event_box = Gtk.EventBox()
            event_box.override_background_color(Gtk.StateFlags.NORMAL, Gdk.RGBA(1, 1, 1, 1))

            vbox = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
            vbox.set_margin_top(16)
            vbox.set_margin_bottom(12)
            vbox.set_margin_start(16)
            vbox.set_margin_end(16)

            loader = GdkPixbuf.PixbufLoader.new_with_type("png")
            loader.write(png_bytes)
            loader.close()
            pixbuf = loader.get_pixbuf()
            gtk_img = Gtk.Image.new_from_pixbuf(pixbuf)
            vbox.pack_start(gtk_img, False, False, 0)

            hint_text = I18n.get("qr_hint")
            hint_label = Gtk.Label(label=hint_text)
            hint_label.override_color(Gtk.StateFlags.NORMAL, Gdk.RGBA(0.18, 0.29, 0.43, 1))
            vbox.pack_start(hint_label, False, False, 4)

            event_box.add(vbox)
            win.add(event_box)

            # Dismiss triggers
            event_box.connect("button-press-event", lambda *_: cls.hide())
            win.connect("button-press-event", lambda *_: cls.hide())
            win.connect("key-press-event", lambda w, e: cls.hide() if e.keyval == Gdk.KEY_Escape else None)

            # Auto-hide timer
            if config.qr_auto_hide_seconds > 0:
                cls._gtk_timer_id = GLib.timeout_add_seconds(config.qr_auto_hide_seconds, cls._hide_gtk)

            win.show_all()
            win.present()
            cls._gtk_window = win
            Logger.log("QRModal: GTK QR modal displayed centered")
            return False

    @classmethod
    def _hide_gtk(cls) -> bool:
        with cls._lock:
            if cls._gtk_timer_id is not None:
                try:
                    from gi.repository import GLib
                    GLib.source_remove(cls._gtk_timer_id)
                except Exception:
                    pass
                cls._gtk_timer_id = None
            if cls._gtk_window is not None:
                try:
                    cls._gtk_window.destroy()
                except Exception:
                    pass
                cls._gtk_window = None
                Logger.log("QRModal: GTK QR modal hidden")
        return False

    # -------------------------------------------------------------------------
    # Fallback Tkinter Implementation (Used if GTK not present)
    # -------------------------------------------------------------------------

    @classmethod
    def _show_tk(cls, config: AppConfig) -> None:
        if not _has_tkinter():
            return
        import tkinter as tk
        from PIL import ImageTk

        with cls._lock:
            if cls._tk_window is not None:
                try:
                    if cls._tk_timer_id:
                        cls._tk_window.after_cancel(cls._tk_timer_id)
                        cls._tk_timer_id = None
                    cls._tk_window.destroy()
                except Exception:
                    pass
                cls._tk_window = None

            orig_qr = cls._fetch_qr_image(config.go_server_url)
            if not orig_qr:
                ServerManager.start_or_attach(config)
                orig_qr = cls._fetch_qr_image(config.go_server_url)
            if not orig_qr:
                Logger.log("QRModal: cannot display QR, local Go server unreachable")
                return

            root = cls._get_tk_root()
            if not root:
                return
            win = tk.Toplevel(root)
            cls._tk_window = win

            win.withdraw()
            win.overrideredirect(True)
            win.attributes("-topmost", True)
            win.configure(bg="#FFFFFF", padx=16, pady=6)

            size = config.qr_popup_size

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

            try:
                resized_qr = orig_qr.resize((size, size), Image.Resampling.NEAREST)
                cls._qr_photo = ImageTk.PhotoImage(resized_qr)
                qr_label = tk.Label(win, image=cls._qr_photo, bg="#FFFFFF", cursor="hand2")
                qr_label.pack(pady=2)
                qr_label.bind("<Button-1>", lambda e: cls.hide())
            except Exception as e:
                Logger.log(f"QRModal: error rendering QR image: {e}")
                win.destroy()
                cls._tk_window = None
                return

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

            win.bind("<Button-1>", lambda e: cls.hide())
            win.bind("<Escape>", lambda e: cls.hide())

            win.update_idletasks()
            win_w = win.winfo_reqwidth()
            win_h = win.winfo_reqheight()
            mon_x, mon_y, screen_w, screen_h = cls._get_active_screen_geometry(win)
            pos_x = mon_x + (screen_w - win_w) // 2
            pos_y = mon_y + (screen_h - win_h) // 2
            win.geometry(f"{win_w}x{win_h}+{pos_x}+{pos_y}")

            win.deiconify()
            win.lift()
            win.focus_force()
            win.update()
            Logger.log(f"QRModal shown at center ({pos_x}, {pos_y})")

            if config.qr_auto_hide_seconds > 0:
                cls._tk_timer_id = win.after(
                    config.qr_auto_hide_seconds * 1000, cls.hide
                )

    @classmethod
    def _hide_tk(cls) -> None:
        with cls._lock:
            if cls._tk_window is not None:
                try:
                    if cls._tk_timer_id:
                        cls._tk_window.after_cancel(cls._tk_timer_id)
                        cls._tk_timer_id = None
                    cls._tk_window.destroy()
                except Exception:
                    pass
                cls._tk_window = None
