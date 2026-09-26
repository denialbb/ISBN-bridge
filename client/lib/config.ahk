#Requires AutoHotkey v2.0

class AppConfig {
    static filePath := ""
    static httpPort := 8766
    static goPort := 8765
    static goServerUrl := "http://127.0.0.1:8765"
    static tokenTtlMinutes := 60
    static targetTabTitle := "hardcover"
    static overwriteExistingText := true
    static autoPasteOnHover := true
    static playTapSoundEnabled := true
    static tooltipOffsetX := 10
    static tooltipOffsetY := 12
    static qrAutoShowOnRefresh := true
    static qrAutoHideSeconds := 45
    static qrPopupSize := 280

    static Init(path := "") {
        this.filePath := (path != "") ? path : this.FindConfigFile()
        this.Load()
    }

    static FindConfigFile() {
        if FileExist(A_ScriptDir "\scanner.conf")
            return A_ScriptDir "\scanner.conf"
        if FileExist(A_ScriptDir "\..\scanner.conf")
            return A_ScriptDir "\..\scanner.conf"
        return A_ScriptDir "\..\scanner.conf"
    }

    static Load() {
        if !FileExist(this.filePath)
            return

        this.httpPort := Integer(IniRead(this.filePath, "Server", "ahk_port", 8766))
        this.goPort := Integer(IniRead(this.filePath, "Server", "port", 8765))
        this.goServerUrl := "http://127.0.0.1:" this.goPort
        this.tokenTtlMinutes := Integer(IniRead(this.filePath, "Server", "token_ttl_minutes", 60))

        this.targetTabTitle := IniRead(this.filePath, "AutoPaste", "target_tab_title", "hardcover")
        this.overwriteExistingText := (IniRead(this.filePath, "AutoPaste", "overwrite_existing_text", "true") = "true")
        this.autoPasteOnHover := (IniRead(this.filePath, "AutoPaste", "auto_paste_on_hover", "true") = "true")
        this.playTapSoundEnabled := (IniRead(this.filePath, "AutoPaste", "play_tap_sound", "true") = "true")

        this.tooltipOffsetX := Integer(IniRead(this.filePath, "AutoPaste", "tooltip_offset_x", 10))
        this.tooltipOffsetY := Integer(IniRead(this.filePath, "AutoPaste", "tooltip_offset_y", 12))

        this.qrAutoShowOnRefresh := (IniRead(this.filePath, "QRCode", "auto_show_on_refresh", "true") = "true")
        this.qrAutoHideSeconds := Integer(IniRead(this.filePath, "QRCode", "auto_hide_seconds", 45))
        this.qrPopupSize := Integer(IniRead(this.filePath, "QRCode", "popup_size", 280))
    }

    static Save(section, key, value) {
        if (this.filePath != "")
            IniWrite(String(value), this.filePath, section, key)
    }
}
