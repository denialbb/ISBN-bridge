#Requires AutoHotkey v2.0

class TrayManager {
    static expiryMenu := Menu()
    static soundMenu := Menu()

    static Init() {
        A_TrayMenu.Delete()

        A_TrayMenu.Add("Attivo", (*) => this.ToggleEnabled())
        A_TrayMenu.Add()

        ; Expiry duration submenu
        this.expiryMenu := Menu()
        this.expiryMenu.Add("15 minuti", (*) => this.SetExpiry(15))
        this.expiryMenu.Add("30 minuti", (*) => this.SetExpiry(30))
        this.expiryMenu.Add("1 ora (60 min)", (*) => this.SetExpiry(60))
        this.expiryMenu.Add("2 ore (120 min)", (*) => this.SetExpiry(120))
        this.expiryMenu.Add("12 ore (720 min)", (*) => this.SetExpiry(720))
        this.expiryMenu.Add("24 ore (1440 min)", (*) => this.SetExpiry(1440))

        A_TrayMenu.Add("Scadenza token", this.expiryMenu)
        A_TrayMenu.Add()

        ; Configurable options
        A_TrayMenu.Add("Sovrascrivi testo (Ctrl+A)", (*) => this.ToggleOverwrite())
        A_TrayMenu.Add("Auto-incolla al passaggio (senza clic)", (*) => this.ToggleAutoHover())

        ; Sound sample submenu
        this.soundMenu := Menu()
        this.soundMenu.Add("Tap (Morbido / Attuale)", (*) => this.SelectSound("tap.wav"))
        this.soundMenu.Add("iOS Tock (Clic felpato)", (*) => this.SelectSound("sounds/ios_tock.wav"))
        this.soundMenu.Add("Bubble Pop (Goccia / Pop morbido)", (*) => this.SelectSound("sounds/bubble_pop.wav"))
        this.soundMenu.Add("Gentle Chime (Accordo marimba)", (*) => this.SelectSound("sounds/gentle_chime.wav"))
        this.soundMenu.Add("Modern Beep (Scanner discreto)", (*) => this.SelectSound("sounds/modern_beep.wav"))
        this.soundMenu.Add("Mechanical Click (Switch tastiera)", (*) => this.SelectSound("sounds/mechanical_click.wav"))
        this.soundMenu.Add("Windows Navigation (Tick classico)", (*) => this.SelectSound("sounds/windows_navigation.wav"))
        this.soundMenu.Add()
        this.soundMenu.Add("Abilita suono", (*) => this.ToggleSound())

        A_TrayMenu.Add("Suono feedback", this.soundMenu)
        A_TrayMenu.Add("Mostra QR code al cambio token", (*) => this.ToggleAutoQR())
        A_TrayMenu.Add()

        ; Quick actions
        A_TrayMenu.Add("Mostra QR code al centro", (*) => QRModal.Show())
        A_TrayMenu.Default := "Mostra QR code al centro"
        A_TrayMenu.ClickCount := 1
        A_TrayMenu.Add("Reset token (Nuovo QR)", (*) => this.ResetToken())
        A_TrayMenu.Add("Apri scanner.conf", (*) => Run(AppConfig.filePath))
        A_TrayMenu.Add("Svuota ISBN in sospeso", (*) => PasteEngine.Cancel())
        A_TrayMenu.Add("Apri log debug", (*) => Logger.Open())
        A_TrayMenu.Add()

        A_TrayMenu.Add("Esci", (*) => ExitApp())

        this.UpdateState()
    }

    static ToggleEnabled() {
        PasteEngine.enabled := !PasteEngine.enabled
        this.UpdateState()
        TrayTip(PasteEngine.enabled ? "Auto-incolla attivo." : "Auto-incolla disattivato.", "ISBN Bridge")
        if !PasteEngine.enabled
            PasteEngine.Cancel()
    }

    static ToggleOverwrite() {
        AppConfig.overwriteExistingText := !AppConfig.overwriteExistingText
        AppConfig.Save("AutoPaste", "overwrite_existing_text", AppConfig.overwriteExistingText ? "true" : "false")
        this.UpdateState()
        TrayTip(AppConfig.overwriteExistingText ? "Sovrascrittura testo attiva." : "Sovrascrittura testo disattivata.", "ISBN Bridge")
    }

    static ToggleAutoHover() {
        AppConfig.autoPasteOnHover := !AppConfig.autoPasteOnHover
        AppConfig.Save("AutoPaste", "auto_paste_on_hover", AppConfig.autoPasteOnHover ? "true" : "false")
        this.UpdateState()
        TrayTip(AppConfig.autoPasteOnHover ? "Auto-incolla al passaggio attivo." : "Auto-incolla al passaggio disattivato.", "ISBN Bridge")
    }

    static ToggleSound() {
        AppConfig.playTapSoundEnabled := !AppConfig.playTapSoundEnabled
        AppConfig.Save("AutoPaste", "play_tap_sound", AppConfig.playTapSoundEnabled ? "true" : "false")
        this.UpdateState()
        TrayTip(AppConfig.playTapSoundEnabled ? "Suono feedback attivo." : "Suono feedback disattivato.", "ISBN Bridge")
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

            TrayTip("Scadenza token impostata a " minutes " minuti.`nNuovo QR generato!", "ISBN Bridge")
            QRModal.Show()
        } catch {
            TrayTip("Scadenza salvata nel file di configurazione.", "ISBN Bridge")
        }
    }

    static ResetToken() {
        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/token/refresh", false)
            req.send()

            TrayTip("Token reimpostato con successo!`nNuovo QR generato.", "ISBN Bridge")
            QRModal.Show()
        } catch as err {
            TrayTip("Errore di contatto col server Go su " AppConfig.goServerUrl, "ISBN Bridge", "Iconx")
        }
    }

    static UpdateState() {
        A_TrayMenu.Uncheck("Attivo")
        if PasteEngine.enabled
            A_TrayMenu.Check("Attivo")

        A_TrayMenu.Uncheck("Sovrascrivi testo (Ctrl+A)")
        if AppConfig.overwriteExistingText
            A_TrayMenu.Check("Sovrascrivi testo (Ctrl+A)")

        A_TrayMenu.Uncheck("Auto-incolla al passaggio (senza clic)")
        if AppConfig.autoPasteOnHover
            A_TrayMenu.Check("Auto-incolla al passaggio (senza clic)")

        this.soundMenu.Uncheck("Abilita suono")
        if AppConfig.playTapSoundEnabled
            this.soundMenu.Check("Abilita suono")

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
            "Tap (Morbido / Attuale)",
            "iOS Tock (Clic felpato)",
            "Bubble Pop (Goccia / Pop morbido)",
            "Gentle Chime (Accordo marimba)",
            "Modern Beep (Scanner discreto)",
            "Mechanical Click (Switch tastiera)",
            "Windows Navigation (Tick classico)"
        ]

        Loop soundFiles.Length {
            this.soundMenu.Uncheck(soundLabels[A_Index])
            if (AppConfig.soundFile = soundFiles[A_Index])
                this.soundMenu.Check(soundLabels[A_Index])
        }

        A_TrayMenu.Uncheck("Mostra QR code al cambio token")
        if AppConfig.qrAutoShowOnRefresh
            A_TrayMenu.Check("Mostra QR code al cambio token")

        options := [15, 30, 60, 120, 720, 1440]
        labels := ["15 minuti", "30 minuti", "1 ora (60 min)", "2 ore (120 min)", "12 ore (720 min)", "24 ore (1440 min)"]

        Loop options.Length {
            this.expiryMenu.Uncheck(labels[A_Index])
            if (AppConfig.tokenTtlMinutes = options[A_Index])
                this.expiryMenu.Check(labels[A_Index])
        }
    }
}
