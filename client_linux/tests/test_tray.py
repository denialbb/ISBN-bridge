"""Tests for Linux TrayManager."""

import sys
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
