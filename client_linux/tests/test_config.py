import os
import tempfile
from client_linux.config import AppConfig

def test_load_default_when_no_file():
    cfg = AppConfig("/non/existent/scanner.conf")
    assert cfg.http_port == 8766
    assert cfg.go_port == 8765
    assert cfg.token_ttl_minutes == 60
    assert cfg.target_tab_title == "hardcover"
    assert cfg.overwrite_existing_text is True
    assert cfg.auto_paste_on_hover is True
    assert cfg.sound_file == "sounds/tap.wav"

def test_load_and_save():
    with tempfile.NamedTemporaryFile("w+", delete=False) as f:
        f.write("""[Server]
port = 9000
ahk_port = 9001
token_ttl_minutes = 45

[AutoPaste]
target_tab_title = mybook
overwrite_existing_text = false
auto_paste_on_hover = false
play_tap_sound = false
sound_file = sounds/bubble_pop.wav

[QRCode]
popup_size = 320
auto_hide_seconds = 60
auto_show_on_refresh = false

[UI]
language = it
""")
        temp_path = f.name

    try:
        cfg = AppConfig(temp_path)
        assert cfg.go_port == 9000
        assert cfg.http_port == 9001
        assert cfg.token_ttl_minutes == 45
        assert cfg.target_tab_title == "mybook"
        assert cfg.overwrite_existing_text is False
        assert cfg.auto_paste_on_hover is False
        assert cfg.play_tap_sound is False
        assert cfg.sound_file == "sounds/bubble_pop.wav"
        assert cfg.qr_popup_size == 320
        assert cfg.qr_auto_hide_seconds == 60
        assert cfg.qr_auto_show_on_refresh is False
        assert cfg.language == "it"

        # Test saving a modified option
        cfg.save("AutoPaste", "overwrite_existing_text", "true")
        cfg.load()
        assert cfg.overwrite_existing_text is True

    finally:
        if os.path.exists(temp_path):
            os.remove(temp_path)
