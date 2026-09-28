import os
import pytest

@pytest.fixture(autouse=True)
def silence_desktop_feedback(monkeypatch):
    """Ensure tests never spam desktop notifications, audio playback, or keystroke injection."""
    monkeypatch.setenv("ISBN_BRIDGE_NO_NOTIFY", "1")
    monkeypatch.setenv("ISBN_BRIDGE_NO_SOUND", "1")
    monkeypatch.setenv("ISBN_BRIDGE_NO_TYPING", "1")
    monkeypatch.setenv("ISBN_BRIDGE_NO_HYPRLAND", "1")
