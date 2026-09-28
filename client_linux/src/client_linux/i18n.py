"""Internationalization (i18n) module loading translations from client/lang/*.ini."""

import configparser
import locale
import os
from pathlib import Path
from typing import Dict, List, Optional


class I18n:
    lang_preference: str = "auto"
    active_lang: str = "en"
    dict_strings: Dict[str, Dict[str, str]] = {}
    lang_names: Dict[str, str] = {}
    lang_dir: Path = Path("client/lang")

    @classmethod
    def init(cls, preference: str = "auto", lang_dir: Optional[Path] = None) -> None:
        cls.lang_dir = lang_dir or cls.find_lang_dir()
        cls.load_languages()
        cls.lang_preference = preference
        cls.resolve_active_language()

    @classmethod
    def find_lang_dir(cls) -> Path:
        pkg_lang = Path(__file__).resolve().parent / "lang"
        candidates = [
            pkg_lang,
            Path("client/lang"),
            Path(__file__).resolve().parent.parent.parent.parent / "client" / "lang",
            Path.cwd() / "client" / "lang",
        ]
        for c in candidates:
            if c.is_dir() and (c / "en.ini").is_file():
                return c.resolve()
        return Path("client/lang")

    @classmethod
    def load_languages(cls) -> None:
        cls.dict_strings = {}
        cls.lang_names = {}

        if not cls.lang_dir.is_dir():
            cls.dict_strings["en"] = {}
            cls.lang_names["en"] = "English"
            return

        for ini_file in cls.lang_dir.glob("*.ini"):
            code = ini_file.stem.lower()
            parser = configparser.ConfigParser(interpolation=None)
            try:
                parser.read(ini_file, encoding="utf-8")
            except Exception:
                continue

            # Load meta
            display_name = code
            if parser.has_section("meta") and parser.has_option("meta", "language_name"):
                display_name = parser.get("meta", "language_name").strip()
            cls.lang_names[code] = display_name

            # Load strings
            strings: Dict[str, str] = {}
            if parser.has_section("strings"):
                for k, v in parser.items("strings"):
                    # Unescape `n and \n
                    val = v.replace("`n", "\n").replace("\\n", "\n")
                    strings[k.lower()] = val
            cls.dict_strings[code] = strings

        if "en" not in cls.dict_strings:
            cls.dict_strings["en"] = {}
            cls.lang_names["en"] = "English"

    @classmethod
    def language_codes(cls) -> List[str]:
        codes = ["en"]
        for c in sorted(cls.dict_strings.keys()):
            if c != "en":
                codes.Push(c) if hasattr(codes, 'Push') else codes.append(c)
        return codes

    @classmethod
    def display_name(cls, code: str) -> str:
        return cls.lang_names.get(code, code)

    @classmethod
    def resolve_active_language(cls) -> None:
        pref = cls.lang_preference.strip().lower()
        if pref and pref != "auto":
            if pref in cls.dict_strings:
                cls.active_lang = pref
                return

        # Auto-detect from system locale
        detected = "en"
        try:
            loc = locale.getlocale()[0] or os.environ.get("LANG", "")
            if loc:
                primary = loc.split("_")[0].lower()
                if primary in cls.dict_strings:
                    detected = primary
        except Exception:
            pass

        cls.active_lang = detected

    @classmethod
    def set_language(cls, new_lang: str) -> None:
        cls.lang_preference = new_lang.strip().lower()
        cls.resolve_active_language()

    @classmethod
    def get(cls, key: str, *params) -> str:
        k = key.lower()
        active = cls.dict_strings.get(cls.active_lang, {})
        en = cls.dict_strings.get("en", {})

        text = active.get(k)
        if text is None:
            text = en.get(k, key)

        # Expand placeholders {1}, {2}, ... (AHK 1-indexed) and {0}, {1}...
        for i, val in enumerate(params, start=1):
            text = text.replace(f"{{{i}}}", str(val))
        for i, val in enumerate(params, start=0):
            text = text.replace(f"{{{i}}}", str(val))

        return text
