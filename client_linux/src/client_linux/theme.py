"""Omarchy theme palette for the QR pairing card.

Reads the active (or explicitly configured) Omarchy theme's colors.toml and
builds a pairing-card palette from it:

- dark themes  -> negative card: accent modules on theme background
- light themes -> positive card: dark ink on theme background
- anything missing/unreadable -> DEFAULT_PALETTE (classic white/navy card)

Theme lookup order for <name>:
1. extra_dirs (e.g. themes/ next to scanner.conf) — custom schemes, also
   the path for non-Omarchy and Windows users
2. $XDG_CONFIG_HOME/omarchy/themes/<name>/ (user themes win)
3. /usr/share/omarchy/themes/<name>/

A setting ending in .toml that points at an existing file is used directly.
"""

import os
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, Iterable, Optional, Tuple

from PIL import Image

from client_linux.logger import Logger

RGB = Tuple[int, int, int]

DEFAULT_INK: RGB = (0x2F, 0x4A, 0x6E)
DEFAULT_BG: RGB = (0xFF, 0xFF, 0xFF)

SYSTEM_THEMES_DIR = Path("/usr/share/omarchy/themes")


@dataclass(frozen=True)
class CardPalette:
    background: RGB  # window chrome + canvas
    foreground: RGB  # hint text
    qr_ink: RGB  # QR modules
    brand_asset: str  # "brand.png" | "brand-dark.png"
    dark: bool


DEFAULT_PALETTE = CardPalette(
    background=DEFAULT_BG,
    foreground=DEFAULT_INK,
    qr_ink=DEFAULT_INK,
    brand_asset="brand.png",
    dark=False,
)


def parse_hex_color(value: object) -> Optional[RGB]:
    """Parse "#RGB" / "#RRGGBB" (tolerates rgba() wrappers by rejecting them)."""
    if not isinstance(value, str):
        return None
    s = value.strip().lstrip("#")
    if len(s) == 3 and all(c in "0123456789abcdefABCDEF" for c in s):
        s = "".join(c * 2 for c in s)
    if len(s) != 6 or any(c not in "0123456789abcdefABCDEF" for c in s):
        return None
    return (int(s[0:2], 16), int(s[2:4], 16), int(s[4:6], 16))


def _read_colors_toml(path: Path) -> Dict[str, str]:
    """Read flat string keys from a colors.toml (tomllib, naive fallback)."""
    try:
        import tomllib

        with open(path, "rb") as f:
            data = tomllib.load(f)
        return {k: v for k, v in data.items() if isinstance(v, str)}
    except ImportError:
        pass
    except Exception as e:
        Logger.log(f"Theme: cannot parse {path}: {e}")
        return {}
    colors: Dict[str, str] = {}
    try:
        with open(path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, _, val = line.partition("=")
                val = val.strip().strip('"').strip("'")
                colors[key.strip()] = val
    except Exception as e:
        Logger.log(f"Theme: cannot read {path}: {e}")
    return colors


def _config_home() -> Path:
    return Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))


def active_theme_name() -> Optional[str]:
    """Name from ~/.config/omarchy/current/theme.name, if present."""
    try:
        p = _config_home() / "omarchy" / "current" / "theme.name"
        if p.is_file():
            name = p.read_text(encoding="utf-8").strip()
            return name or None
    except Exception as e:
        Logger.log(f"Theme: cannot read active theme name: {e}")
    return None


def find_colors_file(theme_name: str, extra_dirs: Iterable[Path] = ()) -> Optional[Path]:
    """Custom drop-in dirs win over Omarchy user themes over system ones."""
    candidates = [Path(d) / theme_name / "colors.toml" for d in extra_dirs]
    candidates += [
        _config_home() / "omarchy" / "themes" / theme_name / "colors.toml",
        SYSTEM_THEMES_DIR / theme_name / "colors.toml",
    ]
    for c in candidates:
        try:
            if c.is_file():
                return c
        except Exception:
            continue
    return None


def palette_for_theme(theme_name: str, extra_dirs: Iterable[Path] = ()) -> Optional[CardPalette]:
    """Build a card palette from a named theme, or None."""
    path = find_colors_file(theme_name, extra_dirs)
    if path is None:
        return None
    return palette_for_file(path)


def palette_for_file(path: Path) -> Optional[CardPalette]:
    """Build a card palette from a colors.toml file, or None."""
    colors = _read_colors_toml(Path(path))
    if not colors:
        return None
    dark = colors.get("mode", "dark").strip().lower() == "dark"
    bg = parse_hex_color(colors.get("background")) or (DEFAULT_BG if not dark else (0, 0, 0))
    fg = parse_hex_color(colors.get("foreground")) or (DEFAULT_INK if not dark else (255, 255, 255))
    accent = parse_hex_color(colors.get("accent"))
    if dark:
        ink = accent or parse_hex_color(colors.get("green")) or fg
        return CardPalette(background=bg, foreground=fg, qr_ink=ink,
                           brand_asset="brand-dark.png", dark=True)
    ink = parse_hex_color(colors.get("dark_foreground")) or fg or DEFAULT_INK
    return CardPalette(background=bg, foreground=ink, qr_ink=ink,
                       brand_asset="brand.png", dark=False)


def load_card_palette(theme_setting: str, extra_dirs: Iterable[Path] = ()) -> CardPalette:
    """Resolve the [QRCode] theme setting to a palette.

    "auto"     -> follow the active Omarchy theme (classic card on light/unknown)
    "default"  -> classic white/navy card
    "<name>"   -> force that theme: custom themes/ dirs first, then Omarchy
    "/path/x"  -> use that colors.toml file directly (ends in .toml, exists)
    """
    setting = (theme_setting or "auto").strip()
    if setting.lower() in ("", "auto"):
        name = active_theme_name()
        if name:
            palette = palette_for_theme(name, extra_dirs)
            if palette is not None:
                return palette
            Logger.log(f"Theme: unknown Omarchy theme '{name}', using default card")
        return DEFAULT_PALETTE
    if setting.lower() == "default":
        return DEFAULT_PALETTE
    if setting.lower().endswith(".toml"):
        try:
            if Path(setting).expanduser().is_file():
                palette = palette_for_file(Path(setting).expanduser())
                if palette is not None:
                    return palette
        except Exception:
            pass
    palette = palette_for_theme(setting.strip().lower(), extra_dirs)
    if palette is None:
        Logger.log(f"Theme: unknown QR theme '{theme_setting}', using default card")
        return DEFAULT_PALETTE
    return palette


def recolor_qr(img: Image.Image, ink: RGB, bg: RGB) -> Image.Image:
    """Repaint QR modules with ink on bg (luminance split, like the server)."""
    src = img.convert("RGB")
    w, h = src.size
    out = Image.new("RGBA", (w, h), bg + (255,))
    src_px = src.load()
    out_px = out.load()
    for y in range(h):
        for x in range(w):
            r, g, b = src_px[x, y]
            lum = 0.299 * r + 0.587 * g + 0.114 * b
            if lum < 127.5:
                out_px[x, y] = ink + (255,)
    return out


def recolor_brand(img: Image.Image, ink: RGB) -> Image.Image:
    """Repaint brand artwork with ink, preserving per-pixel alpha.

    The shipped brand.png is flat navy-on-transparent, so stamping a
    constant ink keeps antialiased edges smooth on any card background.
    """
    src = img.convert("RGBA")
    w, h = src.size
    out = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    src_px = src.load()
    out_px = out.load()
    for y in range(h):
        for x in range(w):
            _, _, _, a = src_px[x, y]
            if a > 16:
                out_px[x, y] = ink + (a,)
    return out
