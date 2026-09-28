from client_linux.i18n import I18n

def test_i18n_loading():
    I18n.init("en")
    assert "en" in I18n.dict_strings
    title = I18n.get("app_title")
    assert title == "ISBN Bridge"

    # Test parameter interpolation {1}
    I18n.init("en")
    msg = I18n.get("isbn_ready_tray", "9781234567890")
    assert "9781234567890" in msg

def test_i18n_fallback():
    I18n.init("unknown_lang")
    assert I18n.get("app_title") == "ISBN Bridge"
