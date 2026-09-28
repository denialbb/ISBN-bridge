"""Tests for Linux TrayManager."""

import sys
import types
from types import SimpleNamespace
from unittest.mock import MagicMock

from client_linux.config import AppConfig
from client_linux.paste import PasteEngine
from client_linux.tray import TrayManager


def test_find_icon():
    cfg = AppConfig()
    engine = PasteEngine(cfg)
    tray = TrayManager(cfg, engine)

    icon_name, theme_dir = tray._find_icon()
    assert icon_name in ("isbn-bridge-symbolic", "tray-symbolic", "tray", "isbn-bridge")
    if icon_name != "isbn-bridge":
        assert theme_dir is not None


def test_on_indicator_activate():
    cfg = AppConfig()
    engine = PasteEngine(cfg)
    tray = TrayManager(cfg, engine)
    tray.show_qr = MagicMock()

    tray._on_indicator_activate(None, 0, 0)
    tray.show_qr.assert_called_once()


def test_tray_toggle_actions():
    cfg = AppConfig()
    engine = PasteEngine(cfg)
    exit_mock = MagicMock()
    tray = TrayManager(cfg, engine, on_exit=exit_mock)

    assert engine.enabled is True
    tray.toggle_enabled(False)
    assert engine.enabled is False
    tray.toggle_enabled(True)
    assert engine.enabled is True

    tray.toggle_overwrite(False)
    assert cfg.overwrite_existing_text is False

    tray.toggle_auto_hover(False)
    assert cfg.auto_paste_on_hover is False

    tray.toggle_sound(False)
    assert cfg.play_tap_sound is False

    tray.toggle_auto_qr(False)
    assert cfg.qr_auto_show_on_refresh is False

    tray.exit_app()
    exit_mock.assert_called_once()


def test_exit_app_from_tray_thread_does_not_raise():
    """The Exit menu item runs in the GTK thread: it must only request
    shutdown (SystemExit there would skip main-thread teardown and the
    interpreter can segfault while the loop thread is still alive)."""
    import threading

    cfg = AppConfig()
    engine = PasteEngine(cfg)
    exit_mock = MagicMock()
    tray = TrayManager(cfg, engine, on_exit=exit_mock)

    errors = []
    t = threading.Thread(target=lambda: _guard(tray.exit_app, errors), daemon=True)
    t.start()
    t.join(timeout=5.0)
    assert not t.is_alive()
    assert not errors, errors[0]
    exit_mock.assert_called_once()


def _guard(fn, errors):
    try:
        fn()
    except BaseException as e:  # noqa: BLE001 - surfaced via assert
        errors.append(e)


def test_join_without_thread_returns():
    cfg = AppConfig()
    tray = TrayManager(cfg, PasteEngine(cfg))
    tray.join(timeout=0.1)  # must not raise


def test_join_waits_for_loop_thread():
    import threading

    cfg = AppConfig()
    tray = TrayManager(cfg, PasteEngine(cfg))
    thread = MagicMock()
    thread.is_alive.return_value = True
    tray._thread = thread
    tray.join(timeout=3.0)
    thread.join.assert_called_once_with(3.0)
    assert tray._thread is None


def test_join_from_loop_thread_itself_never_blocks():
    import threading
    from unittest.mock import patch

    cfg = AppConfig()
    tray = TrayManager(cfg, PasteEngine(cfg))
    thread = MagicMock()
    thread.is_alive.return_value = True
    tray._thread = thread
    with patch.object(threading, "current_thread", return_value=thread):
        tray.join(timeout=3.0)
    thread.join.assert_not_called()
    assert tray._thread is None


def _install_fake_appindicator(monkeypatch):
    """Fake AyatanaAppIndicator3 + Gtk so tray wiring runs headless."""
    indicator = MagicMock(name="Indicator")
    appindicator = SimpleNamespace(
        Indicator=SimpleNamespace(
            new=MagicMock(return_value=indicator),
            new_with_path=MagicMock(return_value=indicator),
        ),
        IndicatorCategory=SimpleNamespace(APPLICATION_STATUS=0),
        IndicatorStatus=SimpleNamespace(ACTIVE=0),
    )
    Gtk = MagicMock(name="Gtk")
    # Distinct widget per constructor call so identity assertions are meaningful.
    menus = []
    Gtk.Menu.side_effect = lambda *a, **k: menus.append(MagicMock(name="Menu")) or menus[-1]
    Gtk.MenuItem.side_effect = lambda *a, **k: MagicMock(name="MenuItem")
    Gtk.CheckMenuItem.side_effect = lambda *a, **k: MagicMock(name="CheckMenuItem")
    Gtk.SeparatorMenuItem.side_effect = lambda *a, **k: MagicMock(name="Separator")

    repo = types.ModuleType("gi.repository")
    repo.AyatanaAppIndicator3 = appindicator
    repo.Gtk = Gtk
    gi_mod = types.ModuleType("gi")
    gi_mod.require_version = MagicMock()
    gi_mod.repository = repo
    monkeypatch.setitem(sys.modules, "gi", gi_mod)
    monkeypatch.setitem(sys.modules, "gi.repository", repo)
    return appindicator, Gtk, indicator, menus


def test_start_wires_menu_and_click_targets(monkeypatch):
    """Right click must get the menu; left/middle click must target Show-QR.

    AyatanaAppIndicator3.Indicator has no "activate" signal: the SNI
    Activate action is delivered via the secondary-activate target, so the
    tray must not rely on connecting to a nonexistent signal.
    """
    appindicator, Gtk, indicator, menus = _install_fake_appindicator(monkeypatch)
    cfg = AppConfig()
    engine = PasteEngine(cfg)
    tray = TrayManager(cfg, engine)

    tray.start()
    tray._thread.join(timeout=5.0)
    assert not tray._thread.is_alive()

    assert appindicator.Indicator.new.call_count + appindicator.Indicator.new_with_path.call_count == 1
    indicator.set_status.assert_called_once()
    top_menu = indicator.set_menu.call_args.args[0]
    assert top_menu in menus
    top_menu.show_all.assert_called_once()
    # Left/middle-click target is the Show-QR item from the menu.
    indicator.set_secondary_activate_target.assert_called_once_with(tray._item_qr)
    # No reliance on the nonexistent Indicator "activate" signal.
    activate_connects = [
        c for c in indicator.connect.call_args_list if c.args and c.args[0] == "activate"
    ]
    assert activate_connects == []
    Gtk.main.assert_called_once()
