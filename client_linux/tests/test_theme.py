"""Tests for Omarchy theme palette resolution and QR recoloring."""

from pathlib import Path

from PIL import Image

from client_linux.theme import (
    DEFAULT_PALETTE,
    CardPalette,
    active_theme_name,
    find_colors_file,
    load_card_palette,
    palette_for_theme,
    parse_hex_color,
    recolor_brand,
    recolor_qr,
)

MATRIX_TOML = """\
mode = "dark"
accent = "#3CBF5C"
background = "#080C09"
foreground = "#8BC98C"
green = "#2D9A48"
"""

LIGHT_TOML = """\
mode = "light"
background = "#FFFFFF"
foreground = "#111111"
dark_foreground = "#222222"
accent = "#2563eb"
"""


def _write_theme(monkeypatch, tmp_path, name="matrix", toml=MATRIX_TOML):
    """Point XDG_CONFIG_HOME at a tmp tree containing one user theme."""
    xdg = tmp_path / "xdg"
    theme_dir = xdg / "omarchy" / "themes" / name
    theme_dir.mkdir(parents=True)
    (theme_dir / "colors.toml").write_text(toml, encoding="utf-8")
    (xdg / "omarchy" / "current").mkdir(parents=True)
    (xdg / "omarchy" / "current" / "theme.name").write_text(name, encoding="utf-8")
    monkeypatch.setenv("XDG_CONFIG_HOME", str(xdg))
    return xdg


def test_parse_hex_color():
    assert parse_hex_color("#3CBF5C") == (60, 191, 92)
    assert parse_hex_color("#fff") == (255, 255, 255)
    assert parse_hex_color("  #080C09  ") == (8, 12, 9)
    assert parse_hex_color("red") is None
    assert parse_hex_color("rgba(60,191,92,1)") is None
    assert parse_hex_color("") is None
    assert parse_hex_color(None) is None


def test_active_theme_name(monkeypatch, tmp_path):
    _write_theme(monkeypatch, tmp_path)
    assert active_theme_name() == "matrix"


def test_find_colors_file_prefers_user_theme(monkeypatch, tmp_path):
    _write_theme(monkeypatch, tmp_path)
    found = find_colors_file("matrix")
    assert found is not None
    assert str(found).startswith(str(tmp_path))
    assert find_colors_file("no-such-theme") is None


def test_palette_for_dark_theme(monkeypatch, tmp_path):
    _write_theme(monkeypatch, tmp_path)
    palette = palette_for_theme("matrix")
    assert palette is not None
    assert palette.dark is True
    assert palette.background == (8, 12, 9)
    assert palette.foreground == (139, 201, 140)
    assert palette.qr_ink == (60, 191, 92)  # accent modules
    assert palette.brand_asset == "brand-dark.png"


def test_palette_for_light_theme(monkeypatch, tmp_path):
    _write_theme(monkeypatch, tmp_path, name="paper", toml=LIGHT_TOML)
    palette = palette_for_theme("paper")
    assert palette is not None
    assert palette.dark is False
    assert palette.background == (255, 255, 255)
    assert palette.brand_asset == "brand.png"


def test_palette_for_missing_theme():
    assert palette_for_theme("no-such-theme-xyz") is None


def test_load_card_palette_settings(monkeypatch, tmp_path):
    _write_theme(monkeypatch, tmp_path)
    assert load_card_palette("default") == DEFAULT_PALETTE
    auto = load_card_palette("auto")
    assert auto.dark is True and auto.qr_ink == (60, 191, 92)
    forced = load_card_palette("matrix")
    assert forced == auto
    assert load_card_palette("no-such-theme-xyz") == DEFAULT_PALETTE
    assert load_card_palette("") == auto


def test_load_card_palette_auto_without_omarchy(monkeypatch, tmp_path):
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "empty"))
    assert load_card_palette("auto") == DEFAULT_PALETTE


def test_recolor_qr_maps_modules_to_ink_and_bg():
    img = Image.new("RGB", (3, 1))
    img.putpixel((0, 0), (47, 74, 110))  # server navy module
    img.putpixel((1, 0), (0, 0, 0))  # pure black module
    img.putpixel((2, 0), (255, 255, 255))  # background
    out = recolor_qr(img, (60, 191, 92), (8, 12, 9))
    assert out.getpixel((0, 0)) == (60, 191, 92, 255)
    assert out.getpixel((1, 0)) == (60, 191, 92, 255)
    assert out.getpixel((2, 0)) == (8, 12, 9, 255)


def test_recolor_brand_stamps_accent_keeping_alpha():
    img = Image.new("RGBA", (3, 1), (0, 0, 0, 0))
    img.putpixel((0, 0), (47, 74, 110, 255))  # navy logo pixel
    img.putpixel((2, 0), (47, 74, 110, 100))  # antialiased edge pixel
    out = recolor_brand(img, (60, 191, 92))
    assert out.getpixel((0, 0)) == (60, 191, 92, 255)
    assert out.getpixel((1, 0))[3] == 0
    assert out.getpixel((2, 0)) == (60, 191, 92, 100)


def _write_custom_dir(tmp_path, name="matrix", accent="#FF0000"):
    d = tmp_path / "custom-themes" / name
    d.mkdir(parents=True)
    (d / "colors.toml").write_text(
        f'mode = "dark"\naccent = "{accent}"\n'
        'background = "#111111"\nforeground = "#EEEEEE"\n',
        encoding="utf-8",
    )
    return tmp_path / "custom-themes"


def test_custom_dir_wins_over_omarchy(monkeypatch, tmp_path):
    _write_theme(monkeypatch, tmp_path)  # omarchy matrix, accent #3CBF5C
    custom = _write_custom_dir(tmp_path)
    palette = load_card_palette("matrix", [custom])
    assert palette.dark is True
    assert palette.qr_ink == (255, 0, 0)
    assert palette.background == (17, 17, 17)


def test_absolute_toml_path_setting(monkeypatch, tmp_path):
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "empty"))
    f = tmp_path / "mine.toml"
    f.write_text(LIGHT_TOML, encoding="utf-8")
    palette = load_card_palette(str(f))
    assert palette.dark is False
    assert palette.background == (255, 255, 255)
    assert load_card_palette(str(tmp_path / "missing.toml")) == DEFAULT_PALETTE


def test_dark_brand_variant_exists():
    from client_linux.ui_qr import QRModal

    found = QRModal._find_brand_asset("brand-dark.png")
    assert found is not None and Path(found).name == "brand-dark.png"
