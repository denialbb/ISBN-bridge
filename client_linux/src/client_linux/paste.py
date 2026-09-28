"""PasteEngine for Linux (Wayland/Hyprland and X11)."""

import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import threading
import time
from typing import Optional, Tuple, Dict, Any

from client_linux.config import AppConfig
from client_linux.i18n import I18n
from client_linux.isbn import validate
from client_linux.logger import Logger
from client_linux.sound import SoundManager
from client_linux.tooltip import FollowToolTip, Notifier


class PasteEngine:
    def __init__(self, config: AppConfig):
        self.config = config
        self.enabled: bool = True
        self.pending_isbn: Optional[str] = None
        self._lock = threading.Lock()
        self._focus_thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()
        self._start_focus_watcher()

    @property
    def is_wayland(self) -> bool:
        return bool(os.environ.get("WAYLAND_DISPLAY") or os.environ.get("XDG_SESSION_TYPE") == "wayland")

    @property
    def is_hyprland(self) -> bool:
        return bool(os.environ.get("HYPRLAND_INSTANCE_SIGNATURE"))

    def matches_target(self, title: str, window_class: str = "") -> bool:
        target = self.config.target_tab_title.strip().lower()
        if not target:
            return True
        return target in title.lower() or target in window_class.lower()

    def get_hyprland_cursor_pos(self) -> Optional[Tuple[int, int]]:
        try:
            res = subprocess.check_output(["hyprctl", "cursorpos"], text=True, timeout=0.5).strip()
            # Format: "x, y"
            parts = res.split(",")
            return int(parts[0].strip()), int(parts[1].strip())
        except Exception:
            return None

    def get_hyprland_active_window(self) -> Optional[Dict[str, Any]]:
        try:
            res = subprocess.check_output(["hyprctl", "activewindow", "-j"], text=True, timeout=0.5)
            data = json.loads(res)
            if isinstance(data, dict) and data.get("title") is not None:
                return data
            return None
        except Exception:
            return None

    def get_hyprland_window_at(self, x: int, y: int) -> Optional[Dict[str, Any]]:
        try:
            res = subprocess.check_output(["hyprctl", "clients", "-j"], text=True, timeout=0.8)
            clients = json.loads(res)
            for c in clients:
                if not c.get("mapped") or c.get("hidden"):
                    continue
                at = c.get("at", [0, 0])
                size = c.get("size", [0, 0])
                wx, wy = at[0], at[1]
                ww, wh = size[0], size[1]
                if wx <= x < wx + ww and wy <= y < wy + wh:
                    return c
            return None
        except Exception:
            return None

    def is_mouse_over_target_field(self) -> bool:
        if self.is_hyprland:
            pos = self.get_hyprland_cursor_pos()
            if pos:
                client = self.get_hyprland_window_at(pos[0], pos[1])
                if client:
                    title = client.get("title", "")
                    wclass = client.get("class", "")
                    return self.matches_target(title, wclass)
            # Fallback to active window
            active = self.get_hyprland_active_window()
            if active:
                return self.matches_target(active.get("title", ""), active.get("class", ""))
            return False

        # X11 fallback
        if shutil.which("xdotool"):
            try:
                res = subprocess.check_output(["xdotool", "getmouselocation"], text=True, timeout=0.5)
                # Format: x:683 y:384 screen:0 window:590
                win_id = None
                for part in res.split():
                    if part.startswith("window:"):
                        win_id = part.split(":")[1]
                if win_id and win_id != "0":
                    title = subprocess.check_output(["xdotool", "getwindowname", win_id], text=True, timeout=0.5).strip()
                    return self.matches_target(title)
            except Exception:
                pass
        return False

    def is_target_window_active(self) -> bool:
        if self.is_hyprland:
            active = self.get_hyprland_active_window()
            if active:
                return self.matches_target(active.get("title", ""), active.get("class", ""))
            return False

        # X11 fallback
        if shutil.which("xdotool"):
            try:
                win_id = subprocess.check_output(["xdotool", "getactivewindow"], text=True, timeout=0.5).strip()
                if win_id:
                    title = subprocess.check_output(["xdotool", "getwindowname", win_id], text=True, timeout=0.5).strip()
                    return self.matches_target(title)
            except Exception:
                pass
        return False

    def arm(self, raw_isbn: str) -> None:
        if not self.enabled:
            Logger.log("PasteEngine: arm called but engine is disabled")
            return

        # 1. Strictly validate ISBN format and mathematical checksum
        isbn = validate(raw_isbn)
        Logger.log(f"PasteEngine: validated ISBN {isbn}")

        # 2. Check if mouse is hovering over target field
        if self.config.auto_paste_on_hover and self.is_mouse_over_target_field():
            Logger.log("PasteEngine: mouse over target window, pasting immediately")
            self.paste_now(isbn, auto_focus_with_click=True)
            return

        # 3. Arm pending ISBN and wait for user click / window focus / trigger
        with self._lock:
            self.pending_isbn = isbn

        msg = I18n.get("isbn_ready_tray", isbn)
        Notifier.notify(msg, I18n.get("app_title"), timeout_ms=4000)
        FollowToolTip.show(I18n.get("isbn_ready_tooltip", isbn))
        Logger.log(f"PasteEngine: armed pending ISBN {isbn}, waiting for user click/focus")

    def trigger(self) -> bool:
        """Trigger immediate paste of pending ISBN."""
        with self._lock:
            isbn = self.pending_isbn

        if not isbn:
            Logger.log("PasteEngine.trigger: no pending ISBN")
            return False

        Logger.log(f"PasteEngine.trigger: executing paste for pending ISBN {isbn}")
        self.paste_now(isbn, auto_focus_with_click=False)
        return True

    def cancel(self) -> None:
        with self._lock:
            self.pending_isbn = None
        FollowToolTip.hide()
        Logger.log("PasteEngine: pending ISBN cleared")

    def paste_now(self, isbn: str, auto_focus_with_click: bool = False) -> None:
        with self._lock:
            self.pending_isbn = None

        FollowToolTip.hide()

        try:
            if auto_focus_with_click:
                time.sleep(0.04)

            # Security guard: verify target window
            if not self.is_target_window_active():
                # If auto-focus was requested or window matches hover, give brief focus chance
                if self.config.target_tab_title and not self.is_target_window_active():
                    Logger.log(f"PasteNow aborted: active window does not match '{self.config.target_tab_title}'")
                    return

            self._inject_keystrokes(isbn)

            SoundManager.play_tap(self.config)
            FollowToolTip.flash(I18n.get("isbn_pasted_tooltip", isbn))
            Logger.log(f"PasteNow successfully pasted ISBN: {isbn}")

        except Exception as e:
            Logger.log(f"PasteNow error: {e}")

    def _inject_keystrokes(self, text: str) -> None:
        # Determine injection strategy
        if self.is_wayland and shutil.which("wtype"):
            self._inject_wayland(text)
        elif shutil.which("xdotool"):
            self._inject_x11(text)
        else:
            Logger.log("Error: Neither wtype nor xdotool found for keystroke injection")

    def _inject_wayland(self, text: str) -> None:
        # 1. Select all if configured
        if self.config.overwrite_existing_text:
            subprocess.run(["wtype", "-M", "ctrl", "-k", "a", "-m", "ctrl"], check=False)
            time.sleep(0.03)

        # 2. Copy to clipboard and paste with Ctrl+V if wl-copy is present
        if shutil.which("wl-copy"):
            p = subprocess.Popen(["wl-copy"], stdin=subprocess.PIPE)
            p.communicate(input=text.encode("utf-8"))
            time.sleep(0.03)
            subprocess.run(["wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl"], check=False)
        else:
            # Type directly
            subprocess.run(["wtype", "--", text], check=False)

    def _inject_x11(self, text: str) -> None:
        # 1. Select all if configured
        if self.config.overwrite_existing_text:
            subprocess.run(["xdotool", "key", "ctrl+a"], check=False)
            time.sleep(0.03)

        # 2. Copy to clipboard and paste with Ctrl+V if xclip is present
        if shutil.which("xclip"):
            p = subprocess.Popen(["xclip", "-selection", "clipboard"], stdin=subprocess.PIPE)
            p.communicate(input=text.encode("utf-8"))
            time.sleep(0.03)
            subprocess.run(["xdotool", "key", "ctrl+v"], check=False)
        else:
            # Type directly
            subprocess.run(["xdotool", "type", "--", text], check=False)

    def _start_focus_watcher(self) -> None:
        """Background watcher that fires deferred paste when user focuses the target window."""
        def run():
            # If on Hyprland, connect to socket2
            sig = os.environ.get("HYPRLAND_INSTANCE_SIGNATURE")
            runtime = os.environ.get("XDG_RUNTIME_DIR", "/run/user/1000")
            sock_path = f"{runtime}/hypr/{sig}/.socket2.sock"

            if self.is_hyprland and sig and os.path.exists(sock_path):
                try:
                    s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                    s.connect(sock_path)
                    s.settimeout(0.5)
                    buf = ""
                    while not self._stop_event.is_set():
                        try:
                            data = s.recv(1024).decode("utf-8", errors="replace")
                            if not data:
                                break
                            buf += data
                            while "\n" in buf:
                                line, buf = buf.split("\n", 1)
                                if line.startswith("activewindow>>"):
                                    self._on_window_focus_event(line)
                        except socket.timeout:
                            continue
                        except Exception:
                            break
                    s.close()
                except Exception as e:
                    Logger.log(f"Hyprland socket2 watcher failed: {e}")

            # Fallback polling loop
            last_active = ""
            while not self._stop_event.is_set():
                if self.pending_isbn:
                    active = self.get_hyprland_active_window() if self.is_hyprland else None
                    title = active.get("title", "") if active else ""
                    if title and title != last_active:
                        last_active = title
                        if self.matches_target(title):
                            time.sleep(0.1)  # Allow browser input focus to settle
                            with self._lock:
                                isbn = self.pending_isbn
                            if isbn:
                                self.paste_now(isbn, auto_focus_with_click=False)
                time.sleep(0.1)

        self._focus_thread = threading.Thread(target=run, daemon=True)
        self._focus_thread.start()

    def _on_window_focus_event(self, event_line: str) -> None:
        """Called when Hyprland emits activewindow>>windowclass,windowtitle."""
        with self._lock:
            isbn = self.pending_isbn
        if not isbn:
            return

        payload = event_line.replace("activewindow>>", "", 1)
        parts = payload.split(",", 1)
        wclass = parts[0] if len(parts) > 0 else ""
        wtitle = parts[1] if len(parts) > 1 else ""

        if self.matches_target(wtitle, wclass):
            Logger.log(f"Focus watcher: target window focused ({wtitle}), triggering deferred paste")
            time.sleep(0.12)  # Give window time to process click/focus
            with self._lock:
                if self.pending_isbn == isbn:
                    self.paste_now(isbn, auto_focus_with_click=False)

    def shutdown(self) -> None:
        self._stop_event.set()
