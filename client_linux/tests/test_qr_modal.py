"""Regression tests for the GTK QR modal.

Bug: QRModal._show_gtk() acquired QRModal._lock and then called
QRModal._hide_gtk(), which acquires the same lock. With a plain
threading.Lock this deadlocked the GTK main-loop thread on the first QR
show, freezing the tray menu (right click) and all click handling
(left click) until restart.
"""

import sys
import threading
import types
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import MagicMock

from PIL import Image

from client_linux.theme import DEFAULT_PALETTE, CardPalette
from client_linux.ui_qr import QRModal


def _install_fake_gi(monkeypatch):
    """Install fake gi.repository modules so GTK code runs headless."""
    Gtk = MagicMock(name="Gtk")
    Gdk = MagicMock(name="Gdk")
    GdkPixbuf = MagicMock(name="GdkPixbuf")
    GLib = MagicMock(name="GLib")
    GLib.timeout_add_seconds.return_value = 1
    Pango = MagicMock(name="Pango")

    repo = types.ModuleType("gi.repository")
    repo.Gtk = Gtk
    repo.Gdk = Gdk
    repo.GdkPixbuf = GdkPixbuf
    repo.GLib = GLib
    repo.Pango = Pango
    gi_mod = types.ModuleType("gi")
    gi_mod.require_version = MagicMock()
    gi_mod.repository = repo

    monkeypatch.setitem(sys.modules, "gi", gi_mod)
    monkeypatch.setitem(sys.modules, "gi.repository", repo)
    return Gtk, Gdk, GdkPixbuf, GLib, Pango


def _stub_qr_sources(monkeypatch):
    monkeypatch.setattr(
        QRModal,
        "_fetch_qr_image",
        classmethod(lambda cls, url: Image.new("RGB", (21, 21), "white")),
    )
    monkeypatch.setattr(QRModal, "_find_brand_asset", classmethod(lambda cls, name="brand.png": None))
    # Hermetic: real palette resolution depends on machine theme files.
    monkeypatch.setattr(
        "client_linux.ui_qr.load_card_palette", lambda setting, extra=(): DEFAULT_PALETTE
    )


def _reset_modal_state():
    QRModal._gtk_window = None
    QRModal._gtk_timer_id = None


def _run_with_timeout(fn, timeout=10.0):
    """Run fn in a thread; fail (don't hang the suite) if it deadlocks."""
    errors = []
    t = threading.Thread(target=_wrap, args=(fn, errors), daemon=True)
    t.start()
    t.join(timeout)
    assert not t.is_alive(), "QRModal GTK call deadlocked (nested _lock acquisition)"
    assert not errors, errors[0]


def _wrap(fn, errors):
    try:
        fn()
    except BaseException as e:  # noqa: BLE001 - re-raised below via assert
        errors.append(e)


def _config():
    return SimpleNamespace(
        go_server_url="http://127.0.0.1:9",
        qr_popup_size=120,
        qr_auto_hide_seconds=0,
    )


def test_modal_lock_is_reentrant():
    """The modal lock must allow _show -> _hide nesting on one thread."""
    acquired_first = QRModal._lock.acquire(blocking=False)
    try:
        assert acquired_first
        assert QRModal._lock.acquire(blocking=False), (
            "QRModal._lock is not reentrant; _show_gtk -> _hide_gtk will deadlock"
        )
    finally:
        QRModal._lock.release()
        if acquired_first:
            QRModal._lock.release()


def test_show_gtk_does_not_deadlock(monkeypatch):
    _install_fake_gi(monkeypatch)
    _stub_qr_sources(monkeypatch)
    _reset_modal_state()
    try:
        _run_with_timeout(lambda: QRModal._show_gtk(_config()))
        assert QRModal._gtk_window is not None
    finally:
        _reset_modal_state()


def test_show_show_hide_cycle_keeps_loop_usable(monkeypatch):
    """Repeated show/hide (menu clicks, auto-show, HTTP /qr/*) must not wedge."""
    _install_fake_gi(monkeypatch)
    _stub_qr_sources(monkeypatch)
    _reset_modal_state()
    try:
        _run_with_timeout(lambda: QRModal._show_gtk(_config()))
        first = QRModal._gtk_window
        assert first is not None
        _run_with_timeout(lambda: QRModal._show_gtk(_config()))
        assert QRModal._gtk_window is not None
        _run_with_timeout(lambda: QRModal._hide_gtk())
        assert QRModal._gtk_window is None
    finally:
        _reset_modal_state()


def test_gtk_matches_ahk_card_metrics(monkeypatch):
    """GTK card metrics must mirror client/lib/ui_qr.ahk.

    AHK: MarginX 16 / MarginY 6, brand at y+2, QR at y+2, hint at y+8,
    hint font s8 Tahoma in brand ink #2F4A6E.
    """
    Gtk, _, _, _, Pango = _install_fake_gi(monkeypatch)
    _stub_qr_sources(monkeypatch)
    _reset_modal_state()
    try:
        _run_with_timeout(lambda: QRModal._show_gtk(_config()))

        vbox = Gtk.Box.return_value
        vbox.set_margin_top.assert_called_with(6)
        vbox.set_margin_bottom.assert_called_with(6)
        vbox.set_margin_start.assert_called_with(16)
        vbox.set_margin_end.assert_called_with(16)

        img = Gtk.Image.new_from_pixbuf.return_value
        img.set_margin_top.assert_called_with(2)

        hint = Gtk.Label.return_value
        hint.set_margin_top.assert_called_with(8)
        Pango.FontDescription.from_string.assert_called_with("Tahoma 8")
        assert hint.modify_font.call_count == 1
    finally:
        _reset_modal_state()


def test_show_gtk_applies_dark_palette(monkeypatch):
    """Themed card: dark canvas/chrome, accent QR, light hint."""
    Gtk, Gdk, _, _, _ = _install_fake_gi(monkeypatch)
    _stub_qr_sources(monkeypatch)
    dark = CardPalette(
        background=(8, 12, 9),
        foreground=(139, 201, 140),
        qr_ink=(60, 191, 92),
        brand_asset="brand-dark.png",
        dark=True,
    )
    monkeypatch.setattr("client_linux.ui_qr.load_card_palette", lambda setting, extra=(): dark)
    # Real brand artwork: exercises open + accent recolor + compose.
    real_brand = Path(__file__).parent.parent / "src" / "client_linux" / "assets" / "brand.png"
    monkeypatch.setattr(
        QRModal, "_find_brand_asset", classmethod(lambda cls, name="brand.png": real_brand)
    )
    _reset_modal_state()
    try:
        _run_with_timeout(lambda: QRModal._show_gtk(_config()))
        assert QRModal._gtk_window is not None
        rgba_calls = [c.args for c in Gdk.RGBA.call_args_list]
        assert (8 / 255, 12 / 255, 9 / 255, 1) in rgba_calls  # chrome bg
        assert (139 / 255, 201 / 255, 140 / 255, 1) in rgba_calls  # hint fg
    finally:
        _reset_modal_state()


def _enable_hyprland(monkeypatch, hyprctl="/usr/bin/hyprctl"):
    monkeypatch.delenv("ISBN_BRIDGE_NO_HYPRLAND", raising=False)
    monkeypatch.setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-sig")
    monkeypatch.setattr("client_linux.ui_qr.shutil.which", lambda _: hyprctl)


def test_hyprland_rule_registers_float_center(monkeypatch):
    """On Hyprland the QR title must get a float+center rule before mapping."""
    import subprocess as sp

    _enable_hyprland(monkeypatch)
    calls = []
    monkeypatch.setattr(
        "client_linux.ui_qr.subprocess.run",
        lambda *a, **k: calls.append(a[0]) or sp.CompletedProcess(a[0], 0, "", ""),
    )
    QRModal._ensure_hyprland_float_rule()
    assert len(calls) == 1
    hyprctl, cmd, lua = calls[0]
    assert cmd == "eval"
    assert "isbn-bridge-qr" in lua
    assert "float = true" in lua
    assert "center = true" in lua
    assert QRModal.QR_TITLE in lua


def test_hyprland_rule_skipped_without_compositor(monkeypatch):
    import subprocess as sp

    monkeypatch.delenv("ISBN_BRIDGE_NO_HYPRLAND", raising=False)
    monkeypatch.delenv("HYPRLAND_INSTANCE_SIGNATURE", raising=False)
    called = []
    monkeypatch.setattr(
        "client_linux.ui_qr.subprocess.run",
        lambda *a, **k: called.append(a[0]) or sp.CompletedProcess(a[0], 0, "", ""),
    )
    QRModal._ensure_hyprland_float_rule()
    assert called == []


def test_hyprland_rule_skipped_when_disabled(monkeypatch):
    import subprocess as sp

    _enable_hyprland(monkeypatch)
    monkeypatch.setenv("ISBN_BRIDGE_NO_HYPRLAND", "1")
    called = []
    monkeypatch.setattr(
        "client_linux.ui_qr.subprocess.run",
        lambda *a, **k: called.append(a[0]) or sp.CompletedProcess(a[0], 0, "", ""),
    )
    QRModal._ensure_hyprland_float_rule()
    assert called == []


def test_hyprland_rule_falls_back_to_legacy_keyword(monkeypatch):
    """Pre-Lua Hyprland (eval fails) gets legacy windowrulev2 keywords."""
    import subprocess as sp

    _enable_hyprland(monkeypatch)
    calls = []

    def fake_run(argv, **kwargs):
        calls.append(argv)
        if argv[1] == "eval":
            return sp.CompletedProcess(argv, 1, "", "unknown request")
        return sp.CompletedProcess(argv, 0, "", "")

    monkeypatch.setattr("client_linux.ui_qr.subprocess.run", fake_run)
    QRModal._ensure_hyprland_float_rule()
    keywords = [c for c in calls if c[1] == "keyword"]
    assert [c[2] for c in keywords] == ["windowrulev2", "windowrulev2"]
    assert any(c[3].startswith("float, title:") for c in keywords)
    assert any(c[3].startswith("center, title:") for c in keywords)


def test_hyprland_rule_never_raises(monkeypatch):
    _enable_hyprland(monkeypatch, hyprctl=None)

    def boom(*a, **k):
        raise OSError("no hyprctl")

    monkeypatch.setattr("client_linux.ui_qr.subprocess.run", boom)
    QRModal._ensure_hyprland_float_rule()  # must not raise
