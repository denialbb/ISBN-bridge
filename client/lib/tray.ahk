#Requires AutoHotkey v2.0

class TrayManager {
    static expiryMenu := Menu()

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
        A_TrayMenu.Add("Suono al tocco (Tap)", (*) => this.ToggleSound())
        A_TrayMenu.Add("Mostra QR code al cambio token", (*) => this.ToggleAutoQR())
        A_TrayMenu.Add()

        ; Quick actions
        A_TrayMenu.Add("Mostra QR code al centro", (*) => QRModal.Show())
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
        TrayTip(PasteEngine.enabled ? "Auto-incolla attivo." : "Auto-incolla disattivato.", "Biblios")
        if !PasteEngine.enabled
            PasteEngine.Cancel()
    }

    static ToggleOverwrite() {
        AppConfig.overwriteExistingText := !AppConfig.overwriteExistingText
        AppConfig.Save("AutoPaste", "overwrite_existing_text", AppConfig.overwriteExistingText ? "true" : "false")
        this.UpdateState()
        TrayTip(AppConfig.overwriteExistingText ? "Sovrascrittura testo attiva." : "Sovrascrittura testo disattivata.", "Biblios")
    }

    static ToggleAutoHover() {
        AppConfig.autoPasteOnHover := !AppConfig.autoPasteOnHover
        AppConfig.Save("AutoPaste", "auto_paste_on_hover", AppConfig.autoPasteOnHover ? "true" : "false")
        this.UpdateState()
        TrayTip(AppConfig.autoPasteOnHover ? "Auto-incolla al passaggio attivo." : "Auto-incolla al passaggio disattivato.", "Biblios")
    }

    static ToggleSound() {
        AppConfig.playTapSoundEnabled := !AppConfig.playTapSoundEnabled
        AppConfig.Save("AutoPaste", "play_tap_sound", AppConfig.playTapSoundEnabled ? "true" : "false")
        this.UpdateState()
        TrayTip(AppConfig.playTapSoundEnabled ? "Suono al tocco attivo." : "Suono al tocco disattivato.", "Biblios")
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

            TrayTip("Scadenza token impostata a " minutes " minuti.`nNuovo QR generato!", "Biblios")
            QRModal.Show()
        } catch {
            TrayTip("Scadenza salvata nel file di configurazione.", "Biblios")
        }
    }

    static ResetToken() {
        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/token/refresh", false)
            req.send()

            TrayTip("Token reimpostato con successo!`nNuovo QR generato.", "Biblios")
            QRModal.Show()
        } catch as err {
            TrayTip("Errore di contatto col server Go su " AppConfig.goServerUrl, "Biblios", "Iconx")
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

        A_TrayMenu.Uncheck("Suono al tocco (Tap)")
        if AppConfig.playTapSoundEnabled
            A_TrayMenu.Check("Suono al tocco (Tap)")

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
