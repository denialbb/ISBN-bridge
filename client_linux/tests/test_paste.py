import pytest
from client_linux.config import AppConfig
from client_linux.paste import PasteEngine
from client_linux.isbn import ISBNValidationError

def test_arm_invalid_isbn_rejected():
    config = AppConfig()
    engine = PasteEngine(config)
    with pytest.raises(ISBNValidationError):
        engine.arm("invalid_isbn_123")

def test_arm_valid_isbn():
    config = AppConfig()
    config.auto_paste_on_hover = False
    engine = PasteEngine(config)
    # Valid ISBN-13
    engine.arm("9780306406157")
    assert engine.pending_isbn == "9780306406157"

def test_cancel():
    config = AppConfig()
    config.auto_paste_on_hover = False
    engine = PasteEngine(config)
    engine.arm("9780306406157")
    assert engine.pending_isbn == "9780306406157"
    engine.cancel()
    assert engine.pending_isbn is None

def test_matches_target():
    config = AppConfig()
    config.target_tab_title = "hardcover"
    engine = PasteEngine(config)

    assert engine.matches_target("Hardcover - Book Details", "firefox") is True
    assert engine.matches_target("My Library", "hardcover-app") is True
    assert engine.matches_target("Wikipedia - Books", "firefox") is False

    # Empty target matches any window
    config.target_tab_title = ""
    assert engine.matches_target("Random Window", "kitty") is True
