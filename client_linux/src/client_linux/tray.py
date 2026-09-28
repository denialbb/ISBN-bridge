"""Tray manager and quick actions for Linux client."""

import glob
import os
from pathlib import Path
import subprocess
import sys
import threading
import urllib.request
from typing import Optional, Tuple

from client_linux.config import AppConfig
from client_linux.i18n import I18n
from client_linux.logger import Logger
from client_linux.paste import PasteEngine
from client_linux.server_manager import ServerManager
from client_linux.sound import SoundManager
from client_linux.tooltip import Notifier
from client_linux.ui_qr import QRModal


def _ensure_gi():
    """Ensure system site-packages matching current Python ABI are in sys.path."""
    abi_path = f"/usr/lib/python{sys.version_info.major}.{sys.version_info.minor}/site-packages"
    if os.path.isdir(abi_path) and abi_path not in sys.path:
        sys.path.append(abi_path)
    debian_path = "/usr/lib/python3/dist-packages"
    if os.path.isdir(debian_path) and debian_path not in sys.path:
        sys.path.append(debian_path)


def _tick_label(base: str, on: bool) -> str:
    """Prefix a "✓ " marker when the option is on.

    The SNI menu exporter (libayatana-appindicator) drops
    GtkCheckMenuItem state: no toggle-type/toggle-state crosses D-Bus
    (verified via GetLayout), so hosts like quickshell can never render
    a native tick. Encoding the state in the label keeps it visible on
    every host. The widget stays a CheckMenuItem so toggle-capable hosts
    keep working if the exporter ever passes state through.
    """
    return ("✓ " if on else "") + base


def _make_check_item(Gtk, base_label: str, is_on: bool, on_toggled):
    """Build a CheckMenuItem whose label tracks its on/off state."""
    item = Gtk.CheckMenuItem(label=_tick_label(base_label, is_on))
    item.set_active(is_on)

    def _on_toggled(widget, _base=base_label, _cb=on_toggled):
        on = widget.get_active()
        widget.set_label(_tick_label(_base, on))
        _cb(on)

    item.connect("toggled", _on_toggled)
    return item


class TrayManager:
    def __init__(self, config: AppConfig, paste_engine: PasteEngine, on_exit=None):
        self.config = config
        self.paste_engine = paste_engine
        self.on_exit = on_exit
        self.indicator = None
        self._thread: Optional[threading.Thread] = None
        self._item_qr = None

    def start(self) -> None:
        """Start tray indicator if supported, or log fallback."""
        _ensure_gi()
        try:
            import gi
            gi.require_version("AyatanaAppIndicator3", "0.1")
            from gi.repository import AyatanaAppIndicator3 as appindicator
            from gi.repository import Gtk
            self._start_appindicator(appindicator, Gtk)
            return
        except Exception as e:
            Logger.log(f"TrayManager: AyatanaAppIndicator3 not available ({e})")

        try:
            import gi
            gi.require_version("AppIndicator3", "0.1")
            from gi.repository import AppIndicator3 as appindicator
            from gi.repository import Gtk
            self._start_appindicator(appindicator, Gtk)
            return
        except Exception as e:
            Logger.log(f"TrayManager: AppIndicator3 not available ({e})")

        Logger.log("TrayManager: No AppIndicator runtime available; running in headless background mode")

    def _start_appindicator(self, appindicator, Gtk) -> None:
        def run():
            icon_name, theme_dir = self._find_icon()
            if theme_dir:
                self.indicator = appindicator.Indicator.new_with_path(
                    "isbn-bridge",
                    icon_name,
                    appindicator.IndicatorCategory.APPLICATION_STATUS,
                    theme_dir,
                )
            else:
                self.indicator = appindicator.Indicator.new(
                    "isbn-bridge",
                    icon_name,
                    appindicator.IndicatorCategory.APPLICATION_STATUS,
                )
            self.indicator.set_status(appindicator.IndicatorStatus.ACTIVE)
            self.indicator.set_title("ISBN Bridge")
            menu = self._build_gtk_menu(Gtk)
            self.indicator.set_menu(menu)

            # Click handling (StatusNotifierItem semantics, verified live and
            # against libayatana-appindicator src/app-indicator.c
            # bus_method_call): the library activates the secondary target
            # ONLY on SecondaryActivate/XAyatanaSecondaryActivate (middle
            # click). A primary Activate (left click) hits the `else` branch
            # there ("unknown method" warning) and is swallowed inside the
            # library: AyatanaAppIndicator3.Indicator exposes zero GObject
            # signals, so there is deliberately nothing to connect here.
            # Quickshell's tray sends activate() on left click (Tray.qml),
            # which is why left click is currently a no-op for this item.
            if hasattr(self, "_item_qr") and self._item_qr:
                try:
                    self.indicator.set_secondary_activate_target(self._item_qr)
                except Exception as e:
                    Logger.log(f"TrayManager: secondary activate not supported: {e}")

            Gtk.main()

        self._thread = threading.Thread(target=run, daemon=True)
        self._thread.start()
        Logger.log("TrayManager: AppIndicator initialized")

    def _find_icon(self) -> Tuple[str, Optional[str]]:
        """Find tray icon, returning (icon_name, theme_dir)."""
        pkg_assets = Path(__file__).resolve().parent / "assets"
        repo_assets = Path(__file__).resolve().parent.parent.parent.parent / "client" / "assets"
        cwd_assets = Path.cwd() / "client" / "assets"

        for asset_dir in [pkg_assets, repo_assets, cwd_assets]:
            for sym_name in ["isbn-bridge-symbolic", "tray-symbolic"]:
                if (asset_dir / f"{sym_name}.png").is_file():
                    return (sym_name, str(asset_dir.resolve()))
            tray_png = asset_dir / "tray.png"
            if tray_png.is_file():
                return ("tray", str(asset_dir.resolve()))

        return ("isbn-bridge-symbolic", None)

    def _build_gtk_menu(self, Gtk):
        menu = Gtk.Menu()

        # Header
        item_title = Gtk.MenuItem(label="ISBN Bridge")
        item_title.set_sensitive(False)
        menu.append(item_title)
        menu.append(Gtk.SeparatorMenuItem())

        # Active Toggle
        item_active = _make_check_item(
            Gtk, I18n.get("tray_active"), self.paste_engine.enabled, self.toggle_enabled
        )
        menu.append(item_active)
        menu.append(Gtk.SeparatorMenuItem())

        # Expiry Submenu
        item_exp = Gtk.MenuItem(label=I18n.get("tray_expiry"))
        exp_menu = Gtk.Menu()
        for mins, label in [
            (15, I18n.get("tray_expiry_min", 15)),
            (30, I18n.get("tray_expiry_min", 30)),
            (60, I18n.get("tray_expiry_hour", 1, 60)),
            (120, I18n.get("tray_expiry_hours", 2, 120)),
            (720, I18n.get("tray_expiry_hours", 12, 720)),
            (1440, I18n.get("tray_expiry_hours", 24, 1440)),
        ]:
            sub = Gtk.MenuItem(label=label)
            sub.connect("activate", lambda w, m=mins: self.set_expiry(m))
            exp_menu.append(sub)
        item_exp.set_submenu(exp_menu)
        menu.append(item_exp)

        # Settings Submenu
        item_settings = Gtk.MenuItem(label=I18n.get("tray_settings"))
        set_menu = Gtk.Menu()

        # Overwrite
        item_ow = _make_check_item(
            Gtk,
            I18n.get("tray_overwrite"),
            self.config.overwrite_existing_text,
            self.toggle_overwrite,
        )
        set_menu.append(item_ow)

        # Auto Hover
        item_ah = _make_check_item(
            Gtk,
            I18n.get("tray_auto_hover"),
            self.config.auto_paste_on_hover,
            self.toggle_auto_hover,
        )
        set_menu.append(item_ah)

        # Sound Enable
        item_snd = _make_check_item(
            Gtk,
            I18n.get("tray_sound_enable"),
            self.config.play_tap_sound,
            self.toggle_sound,
        )
        set_menu.append(item_snd)

        # Auto QR
        item_aqr = _make_check_item(
            Gtk,
            I18n.get("tray_auto_qr"),
            self.config.qr_auto_show_on_refresh,
            self.toggle_auto_qr,
        )
        set_menu.append(item_aqr)

        item_settings.set_submenu(set_menu)
        menu.append(item_settings)
        menu.append(Gtk.SeparatorMenuItem())

        # Quick Actions
        item_qr = Gtk.MenuItem(label=I18n.get("tray_show_qr"))
        item_qr.connect("activate", lambda w: self.show_qr())
        self._item_qr = item_qr
        menu.append(item_qr)

        item_reset = Gtk.MenuItem(label=I18n.get("tray_reset_token"))
        item_reset.connect("activate", lambda w: self.reset_token())
        menu.append(item_reset)

        item_conf = Gtk.MenuItem(label=I18n.get("tray_open_conf"))
        item_conf.connect("activate", lambda w: self.open_conf())
        menu.append(item_conf)

        item_clear = Gtk.MenuItem(label=I18n.get("tray_clear_isbn"))
        item_clear.connect("activate", lambda w: self.clear_pending())
        menu.append(item_clear)

        item_log = Gtk.MenuItem(label=I18n.get("tray_open_log"))
        item_log.connect("activate", lambda w: self.open_log())
        menu.append(item_log)
        menu.append(Gtk.SeparatorMenuItem())

        item_exit = Gtk.MenuItem(label=I18n.get("tray_exit"))
        item_exit.connect("activate", lambda w: self.exit_app())
        menu.append(item_exit)

        menu.show_all()
        return menu

    def toggle_enabled(self, active: Optional[bool] = None) -> None:
        if active is None:
            self.paste_engine.enabled = not self.paste_engine.enabled
        else:
            self.paste_engine.enabled = active
        tip = I18n.get("tray_active_tip_on") if self.paste_engine.enabled else I18n.get("tray_active_tip_off")
        Notifier.notify(tip, I18n.get("app_title"))
        if not self.paste_engine.enabled:
            self.paste_engine.cancel()

    def toggle_overwrite(self, active: Optional[bool] = None) -> None:
        if active is None:
            self.config.overwrite_existing_text = not self.config.overwrite_existing_text
        else:
            self.config.overwrite_existing_text = active
        self.config.save("AutoPaste", "overwrite_existing_text", "true" if self.config.overwrite_existing_text else "false")
        tip = I18n.get("tray_overwrite_tip_on") if self.config.overwrite_existing_text else I18n.get("tray_overwrite_tip_off")
        Notifier.notify(tip, I18n.get("app_title"))

    def toggle_auto_hover(self, active: Optional[bool] = None) -> None:
        if active is None:
            self.config.auto_paste_on_hover = not self.config.auto_paste_on_hover
        else:
            self.config.auto_paste_on_hover = active
        self.config.save("AutoPaste", "auto_paste_on_hover", "true" if self.config.auto_paste_on_hover else "false")
        tip = I18n.get("tray_auto_hover_tip_on") if self.config.auto_paste_on_hover else I18n.get("tray_auto_hover_tip_off")
        Notifier.notify(tip, I18n.get("app_title"))

    def toggle_sound(self, active: Optional[bool] = None) -> None:
        if active is None:
            self.config.play_tap_sound = not self.config.play_tap_sound
        else:
            self.config.play_tap_sound = active
        self.config.save("AutoPaste", "play_tap_sound", "true" if self.config.play_tap_sound else "false")
        tip = I18n.get("tray_sound_tip_on") if self.config.play_tap_sound else I18n.get("tray_sound_tip_off")
        Notifier.notify(tip, I18n.get("app_title"))

    def select_sound(self, sound_path: str) -> None:
        SoundManager.set_sound(self.config, sound_path)

    def toggle_auto_qr(self, active: Optional[bool] = None) -> None:
        if active is None:
            self.config.qr_auto_show_on_refresh = not self.config.qr_auto_show_on_refresh
        else:
            self.config.qr_auto_show_on_refresh = active
        self.config.save("QRCode", "auto_show_on_refresh", "true" if self.config.qr_auto_show_on_refresh else "false")

    def set_expiry(self, minutes: int) -> None:
        self.config.token_ttl_minutes = minutes
        self.config.save("Server", "token_ttl_minutes", str(minutes))
        if not ServerManager.ensure_running(self.config):
            Notifier.notify(I18n.get("tray_reset_token_error", self.config.go_server_url), I18n.get("app_title"), urgency="critical")
            return

        try:
            req = urllib.request.Request(f"{self.config.go_server_url}/token/ttl?minutes={minutes}", method="POST")
            with urllib.request.urlopen(req, timeout=2.0):
                pass
            Notifier.notify(I18n.get("tray_expiry_set_tip", minutes), I18n.get("app_title"))
            QRModal.show(self.config)
        except Exception:
            Notifier.notify(I18n.get("tray_expiry_saved_tip"), I18n.get("app_title"))

    def reset_token(self) -> None:
        if not ServerManager.ensure_running(self.config):
            Notifier.notify(I18n.get("tray_reset_token_error", self.config.go_server_url), I18n.get("app_title"), urgency="critical")
            return

        try:
            req = urllib.request.Request(f"{self.config.go_server_url}/token/refresh", method="POST")
            with urllib.request.urlopen(req, timeout=2.0):
                pass
            Notifier.notify(I18n.get("tray_reset_token_success"), I18n.get("app_title"))
            QRModal.show(self.config)
        except Exception:
            Notifier.notify(I18n.get("tray_reset_token_error", self.config.go_server_url), I18n.get("app_title"), urgency="critical")

    def show_qr(self) -> None:
        QRModal.show(self.config)

    def open_conf(self) -> None:
        try:
            subprocess.Popen(["xdg-open", str(self.config.file_path)])
        except Exception:
            pass

    def clear_pending(self) -> None:
        self.paste_engine.cancel()

    def open_log(self) -> None:
        Logger.open_log()

    def stop(self) -> None:
        try:
            from gi.repository import Gtk, GLib
            GLib.idle_add(Gtk.main_quit)
        except Exception:
            pass

    def join(self, timeout: float = 5.0) -> None:
        """Wait for the tray loop thread to finish (never join self)."""
        thread, self._thread = self._thread, None
        if (
            thread is not None
            and thread.is_alive()
            and thread is not threading.current_thread()
        ):
            thread.join(timeout)

    def exit_app(self) -> None:
        self.stop()
        if self.on_exit:
            self.on_exit()
