#Requires AutoHotkey v2.0

class TrayManager {
    static expiryMenu := Menu()
    static soundMenu := Menu()
    static languageMenu := Menu()
    static consoleMenuItemName := ""

    static GetConsoleMenuLabel() {
        if ServerManager.IsConsoleVisible()
            return I18n.Get("tray_hide_server_console")
        return I18n.Get("tray_show_server_console")
    }

    static UpdateServerMenu() {
        newLabel := this.GetConsoleMenuLabel()
        if (this.consoleMenuItemName != "" && this.consoleMenuItemName != newLabel) {
            try A_TrayMenu.Rename(this.consoleMenuItemName, newLabel)
            this.consoleMenuItemName := newLabel
        }
    }

    static Init() {
        A_TrayMenu.Delete()
        A_IconTip := I18n.Get("tray_icon_tip")

        ; Custom tray icon (falls back to the AutoHotkey default if missing)
        try {
            iconPath := A_ScriptDir "\assets\tray.ico"
            if FileExist(iconPath)
                TraySetIcon(iconPath)
        }

        A_TrayMenu.Add(I18n.Get("tray_active"), (*) => this.ToggleEnabled())
        A_TrayMenu.Add()

        ; Expiry duration submenu
        this.expiryMenu := Menu()
        this.expiryMenu.Add(I18n.Get("tray_expiry_min", 15), (*) => this.SetExpiry(15))
        this.expiryMenu.Add(I18n.Get("tray_expiry_min", 30), (*) => this.SetExpiry(30))
        this.expiryMenu.Add(I18n.Get("tray_expiry_hour", 1, 60), (*) => this.SetExpiry(60))
        this.expiryMenu.Add(I18n.Get("tray_expiry_hours", 2, 120), (*) => this.SetExpiry(120))
        this.expiryMenu.Add(I18n.Get("tray_expiry_hours", 12, 720), (*) => this.SetExpiry(720))
        this.expiryMenu.Add(I18n.Get("tray_expiry_hours", 24, 1440), (*) => this.SetExpiry(1440))

        A_TrayMenu.Add(I18n.Get("tray_expiry"), this.expiryMenu)
        A_TrayMenu.Add()

        ; Configurable options
        A_TrayMenu.Add(I18n.Get("tray_overwrite"), (*) => this.ToggleOverwrite())
        A_TrayMenu.Add(I18n.Get("tray_auto_hover"), (*) => this.ToggleAutoHover())

        ; Sound sample submenu
        this.soundMenu := Menu()
        this.soundMenu.Add(I18n.Get("sound_tap"), (*) => this.SelectSound("tap.wav"))
        this.soundMenu.Add(I18n.Get("sound_ios"), (*) => this.SelectSound("sounds/ios_tock.wav"))
        this.soundMenu.Add(I18n.Get("sound_bubble"), (*) => this.SelectSound("sounds/bubble_pop.wav"))
        this.soundMenu.Add(I18n.Get("sound_chime"), (*) => this.SelectSound("sounds/gentle_chime.wav"))
        this.soundMenu.Add(I18n.Get("sound_beep"), (*) => this.SelectSound("sounds/modern_beep.wav"))
        this.soundMenu.Add(I18n.Get("sound_click"), (*) => this.SelectSound("sounds/mechanical_click.wav"))
        this.soundMenu.Add(I18n.Get("sound_nav"), (*) => this.SelectSound("sounds/windows_navigation.wav"))
        this.soundMenu.Add()
        this.soundMenu.Add(I18n.Get("tray_sound_enable"), (*) => this.ToggleSound())

        A_TrayMenu.Add(I18n.Get("tray_sound"), this.soundMenu)
        A_TrayMenu.Add(I18n.Get("tray_auto_qr"), (*) => this.ToggleAutoQR())
        A_TrayMenu.Add()

        ; Language submenu
        this.languageMenu := Menu()
        this.languageMenu.Add(I18n.Get("lang_auto"), (*) => I18n.SetLanguage("auto"))
        this.languageMenu.Add()
        this.languageMenu.Add(I18n.Get("lang_it"), (*) => I18n.SetLanguage("it"))
        this.languageMenu.Add(I18n.Get("lang_en"), (*) => I18n.SetLanguage("en"))

        A_TrayMenu.Add(I18n.Get("tray_language"), this.languageMenu)
        A_TrayMenu.Add()

        ; Quick actions
        showQRLabel := I18n.Get("tray_show_qr")
        A_TrayMenu.Add(showQRLabel, (*) => QRModal.Show())
        A_TrayMenu.Default := showQRLabel
        A_TrayMenu.ClickCount := 1

        this.consoleMenuItemName := this.GetConsoleMenuLabel()
        A_TrayMenu.Add(this.consoleMenuItemName, (*) => ServerManager.ToggleConsole())

        A_TrayMenu.Add(I18n.Get("tray_reset_token"), (*) => this.ResetToken())
        A_TrayMenu.Add(I18n.Get("tray_open_conf"), (*) => Run(AppConfig.filePath))
        A_TrayMenu.Add(I18n.Get("tray_clear_isbn"), (*) => PasteEngine.Cancel())
        A_TrayMenu.Add(I18n.Get("tray_open_log"), (*) => Logger.Open())
        A_TrayMenu.Add()

        A_TrayMenu.Add(I18n.Get("tray_exit"), (*) => ExitApp())

        this.UpdateState()
    }

    static ToggleEnabled() {
        PasteEngine.enabled := !PasteEngine.enabled
        this.UpdateState()
        tipMsg := PasteEngine.enabled ? I18n.Get("tray_active_tip_on") : I18n.Get("tray_active_tip_off")
        TrayTip(tipMsg, I18n.Get("app_title"))
        if !PasteEngine.enabled
            PasteEngine.Cancel()
    }

    static ToggleOverwrite() {
        AppConfig.overwriteExistingText := !AppConfig.overwriteExistingText
        AppConfig.Save("AutoPaste", "overwrite_existing_text", AppConfig.overwriteExistingText ? "true" : "false")
        this.UpdateState()
        tipMsg := AppConfig.overwriteExistingText ? I18n.Get("tray_overwrite_tip_on") : I18n.Get("tray_overwrite_tip_off")
        TrayTip(tipMsg, I18n.Get("app_title"))
    }

    static ToggleAutoHover() {
        AppConfig.autoPasteOnHover := !AppConfig.autoPasteOnHover
        AppConfig.Save("AutoPaste", "auto_paste_on_hover", AppConfig.autoPasteOnHover ? "true" : "false")
        this.UpdateState()
        tipMsg := AppConfig.autoPasteOnHover ? I18n.Get("tray_auto_hover_tip_on") : I18n.Get("tray_auto_hover_tip_off")
        TrayTip(tipMsg, I18n.Get("app_title"))
    }

    static ToggleSound() {
        AppConfig.playTapSoundEnabled := !AppConfig.playTapSoundEnabled
        AppConfig.Save("AutoPaste", "play_tap_sound", AppConfig.playTapSoundEnabled ? "true" : "false")
        this.UpdateState()
        tipMsg := AppConfig.playTapSoundEnabled ? I18n.Get("tray_sound_tip_on") : I18n.Get("tray_sound_tip_off")
        TrayTip(tipMsg, I18n.Get("app_title"))
    }

    static SelectSound(path) {
        SoundManager.SetSound(path)
        this.UpdateState()
    }

    static ToggleAutoQR() {
        AppConfig.qrAutoShowOnRefresh := !AppConfig.qrAutoShowOnRefresh
        AppConfig.Save("QRCode", "auto_show_on_refresh", AppConfig.qrAutoShowOnRefresh ? "true" : "false")
        this.UpdateState()
    }

    static SetExpiry(minutes) {
        AppConfig.tokenTtlMinutes := minutes
        AppConfig.Save("Server", "token_ttl_minutes", minutes)
        this.UpdateState()

        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/token/ttl?minutes=" minutes, false)
            req.send()

            TrayTip(I18n.Get("tray_expiry_set_tip", minutes), I18n.Get("app_title"))
            QRModal.Show()
        } catch {
            TrayTip(I18n.Get("tray_expiry_saved_tip"), I18n.Get("app_title"))
        }
    }

    static ResetToken() {
        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/token/refresh", false)
            req.send()

            TrayTip(I18n.Get("tray_reset_token_success"), I18n.Get("app_title"))
            QRModal.Show()
        } catch as err {
            TrayTip(I18n.Get("tray_reset_token_error", AppConfig.goServerUrl), I18n.Get("app_title"), "Iconx")
        }
    }

    static UpdateState() {
        A_TrayMenu.Uncheck(I18n.Get("tray_active"))
        if PasteEngine.enabled
            A_TrayMenu.Check(I18n.Get("tray_active"))

        A_TrayMenu.Uncheck(I18n.Get("tray_overwrite"))
        if AppConfig.overwriteExistingText
            A_TrayMenu.Check(I18n.Get("tray_overwrite"))

        A_TrayMenu.Uncheck(I18n.Get("tray_auto_hover"))
        if AppConfig.autoPasteOnHover
            A_TrayMenu.Check(I18n.Get("tray_auto_hover"))

        this.soundMenu.Uncheck(I18n.Get("tray_sound_enable"))
        if AppConfig.playTapSoundEnabled
            this.soundMenu.Check(I18n.Get("tray_sound_enable"))

        soundFiles := [
            "tap.wav",
            "sounds/ios_tock.wav",
            "sounds/bubble_pop.wav",
            "sounds/gentle_chime.wav",
            "sounds/modern_beep.wav",
            "sounds/mechanical_click.wav",
            "sounds/windows_navigation.wav"
        ]
        soundLabels := [
            I18n.Get("sound_tap"),
            I18n.Get("sound_ios"),
            I18n.Get("sound_bubble"),
            I18n.Get("sound_chime"),
            I18n.Get("sound_beep"),
            I18n.Get("sound_click"),
            I18n.Get("sound_nav")
        ]

        Loop soundFiles.Length {
            this.soundMenu.Uncheck(soundLabels[A_Index])
            if (AppConfig.soundFile = soundFiles[A_Index])
                this.soundMenu.Check(soundLabels[A_Index])
        }

        ; Language checkmarks
        this.languageMenu.Uncheck(I18n.Get("lang_auto"))
        this.languageMenu.Uncheck(I18n.Get("lang_it"))
        this.languageMenu.Uncheck(I18n.Get("lang_en"))

        if (I18n.langPreference = "auto")
            this.languageMenu.Check(I18n.Get("lang_auto"))
        else if (I18n.langPreference = "it")
            this.languageMenu.Check(I18n.Get("lang_it"))
        else if (I18n.langPreference = "en")
            this.languageMenu.Check(I18n.Get("lang_en"))

        A_TrayMenu.Uncheck(I18n.Get("tray_auto_qr"))
        if AppConfig.qrAutoShowOnRefresh
            A_TrayMenu.Check(I18n.Get("tray_auto_qr"))

        options := [15, 30, 60, 120, 720, 1440]
        labels := [
            I18n.Get("tray_expiry_min", 15),
            I18n.Get("tray_expiry_min", 30),
            I18n.Get("tray_expiry_hour", 1, 60),
            I18n.Get("tray_expiry_hours", 2, 120),
            I18n.Get("tray_expiry_hours", 12, 720),
            I18n.Get("tray_expiry_hours", 24, 1440)
        ]

        Loop options.Length {
            this.expiryMenu.Uncheck(labels[A_Index])
            if (AppConfig.tokenTtlMinutes = options[A_Index])
                this.expiryMenu.Check(labels[A_Index])
        }

        this.UpdateServerMenu()
    }
}
