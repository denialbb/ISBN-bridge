#Requires AutoHotkey v2.0

; QR card theming shared with the Linux client (client_linux/theme.py).
;
; [QRCode] theme= setting: auto | default | <name> | <path to colors.toml>
;   auto     -> default card (no Omarchy on Windows)
;   default  -> classic white/navy card
;   <name>   -> <config-dir>\themes\<name>\colors.toml drop-in scheme
;   <path>   -> that colors.toml file directly
; colors.toml keys: mode ("dark"/"light"), accent, background, foreground,
;                   green, dark_foreground. Missing mode is inferred from
;                   background luminance. Anything unreadable -> default card.

class QRTheme {
    static Classic() {
        return Map("bg", "FFFFFF", "fg", "2F4A6E", "ink", "2F4A6E",
            "dark", false, "brand", "brand.png")
    }

    static Load(setting := "auto") {
        s := Trim(StrLower(setting))
        if (s = "" || s = "auto" || s = "default")
            return this.Classic()
        if (SubStr(s, -4) = ".toml" && FileExist(setting)) {
            p := this.FromFile(setting)
            if (p.Count)
                return p
            return this.Classic()
        }
        confDir := ""
        SplitPath(AppConfig.filePath, , &confDir)
        if (confDir != "") {
            p := this.FromFile(confDir "\themes\" s "\colors.toml")
            if (p.Count)
                return p
        }
        Logger.Log("QRTheme: unknown theme '" setting "', using default card")
        return this.Classic()
    }

    static FromFile(path) {
        colors := Map()
        try {
            text := FileRead(path, "UTF-8")
            Loop Parse, text, "`n", "`r" {
                line := Trim(A_LoopField)
                if (line = "" || SubStr(line, 1, 1) = "#" || !InStr(line, "="))
                    continue
                parts := StrSplit(line, "=", , 2)
                key := StrLower(Trim(parts[1]))
                val := Trim(parts[2])
                if (SubStr(val, 1, 1) = '"' && SubStr(val, -1) = '"')
                    val := SubStr(val, 2, -1)
                colors[key] := val
            }
        } catch {
            return Map()
        }
        if (!colors.Count)
            return Map()
        mode := colors.Has("mode") ? StrLower(colors["mode"]) : ""
        bgRaw := colors.Has("background") ? colors["background"] : ""
        dark := (mode != "") ? (mode = "dark") : this.IsDark(this.CleanHex(bgRaw, "FFFFFF"))
        if (dark) {
            bg := this.CleanHex(bgRaw, "080C09")
            fg := this.CleanHex(colors.Get("foreground", ""), "D6E2EE")
            ink := this.CleanHex(colors.Get("accent", ""), "")
            if (ink = "")
                ink := this.CleanHex(colors.Get("green", ""), fg)
            return Map("bg", bg, "fg", fg, "ink", ink, "dark", true, "brand", "brand-dark.png")
        }
        bg := this.CleanHex(bgRaw, "FFFFFF")
        ink := this.CleanHex(colors.Get("dark_foreground", ""), "")
        if (ink = "")
            ink := this.CleanHex(colors.Get("foreground", ""), "2F4A6E")
        return Map("bg", bg, "fg", ink, "ink", ink, "dark", false, "brand", "brand.png")
    }

    static CleanHex(val, fallback) {
        v := Trim(val)
        if (SubStr(v, 1, 1) = "#")
            v := SubStr(v, 2)
        if (RegExMatch(v, "^[0-9a-fA-F]{6}$"))
            return StrUpper(v)
        return fallback
    }

    static IsDark(hex6) {
        r := Integer("0x" SubStr(hex6, 1, 2))
        g := Integer("0x" SubStr(hex6, 3, 2))
        b := Integer("0x" SubStr(hex6, 5, 2))
        return (0.299 * r + 0.587 * g + 0.114 * b) < 127.5
    }
}
