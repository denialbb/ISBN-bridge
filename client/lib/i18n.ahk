#Requires AutoHotkey v2.0

; Data-driven UI strings. Translations live in lang/*.ini next to the
; executable (compiled release) or under client/lang (dev). To add a
; language, drop in one INI file — see client/lang/en.ini for the format.
; English (en.ini) is required and is the fallback for missing keys.

class I18n {
    static langPreference := "auto"
    static activeLang := "en"
    static dict := Map()      ; code -> Map(key -> text)
    static langNames := Map() ; code -> display name from [meta]
    static langIds := Map()   ; code -> primary OS language ID from [meta]
    static langDir := ""

    static Init() {
        this.langDir := this.FindLangDir()
        this.LoadLanguages()
        this.langPreference := (AppConfig.language != "") ? AppConfig.language : "auto"
        this.ResolveActiveLanguage()
    }

    static FindLangDir() {
        ; Test hook: point at a scratch dir without touching the repo.
        if (override := EnvGet("ISBN_BRIDGE_LANG_DIR")) != "" && FileExist(override "\en.ini")
            return override
        ; Compiled release layout is flat (lang/ next to the exe).
        if FileExist(A_ScriptDir "\lang\en.ini")
            return A_ScriptDir "\lang"
        ; Dev layouts.
        if FileExist(A_ScriptDir "\client\lang\en.ini")
            return A_ScriptDir "\client\lang"
        if FileExist(A_ScriptDir "\..\client\lang\en.ini")
            return A_ScriptDir "\..\client\lang"
        return A_ScriptDir "\lang"
    }

    static DiscoverCodes() {
        codes := ["en"]
        Loop Files, this.langDir "\*.ini" {
            name := A_LoopFileName
            ext := A_LoopFileExt
            code := StrLower(SubStr(name, 1, StrLen(name) - StrLen(ext) - 1))
            if (code != "en" && RegExMatch(code, "^[a-z]{2,5}$"))
                codes.Push(code)
        }
        return codes
    }

    ; Custom INI reader. The Win32 INI API (IniRead) silently drops the
    ; first section when a file starts with a UTF-8 BOM — which is what
    ; Notepad writes by default — so translators' files would lose [meta].
    ; Explicit UTF-8: without it, BOM-less files decode as ANSI and every
    ; non-ASCII string (bullet, accents) renders as mojibake. The manual
    ; BOM strip below covers editors that add one.
    static LoadIniFile(path) {
        sections := Map()
        try
            text := FileRead(path, "UTF-8")
        catch
            return sections
        if (SubStr(text, 1, 1) = Chr(0xFEFF))
            text := SubStr(text, 2)
        current := ""
        Loop Parse, text, "`n", "`r" {
            line := Trim(A_LoopField)
            if (line = "" || SubStr(line, 1, 1) = ";" || SubStr(line, 1, 1) = "#")
                continue
            if (RegExMatch(line, "^\[(.+)\]$", &m)) {
                current := StrLower(Trim(m[1]))
                if !sections.Has(current)
                    sections[current] := Map()
                continue
            }
            if (current = "")
                continue
            pos := InStr(line, "=")
            if (pos < 2)
                continue
            key := StrLower(Trim(SubStr(line, 1, pos - 1)))
            sections[current][key] := Trim(SubStr(line, pos + 1))
        }
        return sections
    }

    static LoadLanguages() {
        this.dict := Map()
        this.langNames := Map()
        this.langIds := Map()
        for code in this.DiscoverCodes() {
            ini := this.LoadIniFile(this.langDir "\" code ".ini")
            meta := ini.Has("meta") ? ini["meta"] : Map()
            if (meta.Has("language_name") && Trim(meta["language_name"]) != "")
                this.langNames[code] := Trim(meta["language_name"])
            else
                this.langNames[code] := code
            id := 0
            try id := Integer(meta.Has("lang_id") ? meta["lang_id"] : "0")
            catch
                id := 0
            this.langIds[code] := id
            this.dict[code] := ini.Has("strings") ? ini["strings"] : Map()
        }
        if !this.dict.Has("en") {
            this.dict["en"] := Map()
            this.langNames["en"] := "English"
            this.langIds["en"] := 9
        }
    }

    ; Codes with English first, for the tray language menu.
    static LanguageCodes() {
        codes := ["en"]
        for code in this.dict {
            if (code != "en")
                codes.Push(code)
        }
        return codes
    }

    static DisplayName(code) {
        return this.langNames.Has(code) ? this.langNames[code] : code
    }

    static ResolveActiveLanguage() {
        pref := StrLower(Trim(String(this.langPreference)))
        if (pref != "" && pref != "auto") {
            if this.dict.Has(pref) {
                this.activeLang := pref
                return
            }
            Logger.Log("Unknown language '" pref "' in scanner.conf; falling back to auto-detect.")
        }
        ; Auto: match the OS UI language against each file's lang_id.
        langId := DllCall("Kernel32.dll\GetUserDefaultUILanguage", "UShort")
        primary := langId & 0x3FF
        this.activeLang := "en"
        for code, id in this.langIds {
            if (id != 0 && id = primary) {
                this.activeLang := code
                break
            }
        }
    }

    static SetLanguage(newLang) {
        newLang := StrLower(Trim(String(newLang)))
        if (newLang != "auto" && !this.dict.Has(newLang))
            newLang := "auto"
        this.langPreference := newLang
        AppConfig.language := newLang
        AppConfig.Save("UI", "language", newLang)
        this.ResolveActiveLanguage()

        ; Rebuild Tray menu with updated strings
        TrayManager.Init()
        TrayTip(this.Get("lang_switched", this.DisplayName(this.activeLang)), "ISBN Bridge")
    }

    static Get(key, params*) {
        if !this.dict.Has(this.activeLang) || !this.dict[this.activeLang].Has(key) {
            ; Fallback to English
            if this.dict.Has("en") && this.dict["en"].Has(key)
                text := this.dict["en"][key]
            else
                return key
        } else {
            text := this.dict[this.activeLang][key]
        }

        ; INI files store newlines as literal `n; expand them here.
        text := StrReplace(text, "``n", "`n")
        for i, val in params {
            text := StrReplace(text, "{" i "}", String(val))
        }
        return text
    }
}
