#Requires AutoHotkey v2.0

; Private-font loader: makes bundled TTFs (e.g. Fira Code) usable without
; installing them system-wide and without admin rights. FR_PRIVATE (0x10)
; keeps the font visible to this process only; it is unloaded on exit.
class FontLoader {
    static loaded := []

    static Load() {
        fonts := [
            A_ScriptDir "\assets\fonts\FiraCode-Regular.ttf",
            A_ScriptDir "\assets\fonts\FiraCode-Bold.ttf"
        ]
        for path in fonts {
            if !FileExist(path)
                continue
            try {
                added := DllCall("Gdi32.dll\AddFontResourceExW", "WStr", path, "UInt", 0x10, "Ptr", 0, "Int")
                if (added > 0) {
                    this.loaded.Push(path)
                    Logger.Log("Font loaded: " path)
                } else {
                    Logger.Log("Font already available or failed: " path)
                }
            } catch as err {
                Logger.Log("Font load failed (" path "): " err.Message)
            }
        }
    }

    static Unload() {
        for path in this.loaded {
            try {
                DllCall("Gdi32.dll\RemoveFontResourceExW", "WStr", path, "UInt", 0x10, "Ptr", 0)
            }
        }
        this.loaded := []
    }
}
