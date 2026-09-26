#Requires AutoHotkey v2.0

class I18n {
    static langPreference := "auto"
    static activeLang := "en"
    static dict := Map()

    static Init() {
        this.SetupDictionary()
        this.langPreference := (AppConfig.language != "") ? AppConfig.language : "auto"
        this.ResolveActiveLanguage()
    }

    static ResolveActiveLanguage() {
        if (this.langPreference = "it") {
            this.activeLang := "it"
        } else if (this.langPreference = "en") {
            this.activeLang := "en"
        } else {
            ; Auto-detect OS UI language (0x10 = LANG_ITALIAN, 0x0410 = it-IT)
            langId := DllCall("Kernel32.dll\GetUserDefaultUILanguage", "UShort")
            primaryLang := langId & 0x3FF
            this.activeLang := (primaryLang = 0x10 || A_Language = "0410") ? "it" : "en"
        }
    }

    static SetLanguage(newLang) {
        this.langPreference := newLang
        AppConfig.language := newLang
        AppConfig.Save("UI", "language", newLang)
        this.ResolveActiveLanguage()

        ; Rebuild Tray menu with updated strings
        TrayManager.Init()
        TrayTip(this.Get("lang_switched"), "ISBN Bridge")
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

        for i, val in params {
            text := StrReplace(text, "{" i "}", String(val))
        }
        return text
    }

    static SetupDictionary() {
        this.dict["it"] := Map(
            "app_title", "ISBN Bridge",
            "lang_switched", "Lingua impostata su: Italiano",
            "client_active_tip", "Client ISBN Bridge attivo (Porta: {1})",
            "tray_icon_tip", "Clicca per mostrare il QR code",
            "listener_error_title", "ISBN Bridge - Errore",
            "listener_error_msg", "Impossibile avviare il listener AutoHotkey sulla porta {1}.`n`nLa porta potrebbe essere già in uso.",
            "qr_hint", "Inquadra per abbinare • Clic o ESC per chiudere",
            "qr_download_error", "Impossibile scaricare il QR code dal server Go.",
            "isbn_ready_tooltip", "ISBN pronto: {1}`nClicca nel campo di testo per incollarlo.`nESC per annullare.",
            "isbn_ready_tray", "ISBN pronto: {1}",
            "isbn_pasted_tooltip", "ISBN incollato: {1}",
            "tray_active", "Attivo",
            "tray_active_tip_on", "Auto-incolla attivo.",
            "tray_active_tip_off", "Auto-incolla disattivato.",
            "tray_expiry", "Scadenza token",
            "tray_expiry_min", "{1} minuti",
            "tray_expiry_hour", "{1} ora ({2} min)",
            "tray_expiry_hours", "{1} ore ({2} min)",
            "tray_expiry_set_tip", "Scadenza token impostata a {1} minuti.`nNuovo QR generato!",
            "tray_expiry_saved_tip", "Scadenza salvata nel file di configurazione.",
            "tray_overwrite", "Sovrascrivi testo (Ctrl+A)",
            "tray_overwrite_tip_on", "Sovrascrittura testo attiva.",
            "tray_overwrite_tip_off", "Sovrascrittura testo disattivata.",
            "tray_auto_hover", "Auto-incolla al passaggio (senza clic)",
            "tray_auto_hover_tip_on", "Auto-incolla al passaggio attivo.",
            "tray_auto_hover_tip_off", "Auto-incolla al passaggio disattivato.",
            "tray_sound", "Suono feedback",
            "tray_sound_enable", "Abilita suono",
            "tray_sound_tip_on", "Suono feedback attivo.",
            "tray_sound_tip_off", "Suono feedback disattivato.",
            "sound_tap", "Tap (Morbido / Attuale)",
            "sound_ios", "iOS Tock (Clic felpato)",
            "sound_bubble", "Bubble Pop (Goccia / Pop morbido)",
            "sound_chime", "Gentle Chime (Accordo marimba)",
            "sound_beep", "Modern Beep (Scanner discreto)",
            "sound_click", "Mechanical Click (Switch tastiera)",
            "sound_nav", "Windows Navigation (Tick classico)",
            "tray_auto_qr", "Mostra QR code al cambio token",
            "tray_show_qr", "Mostra QR code al centro",
            "tray_reset_token", "Reset token (Nuovo QR)",
            "tray_reset_token_success", "Token reimpostato con successo!`nNuovo QR generato.",
            "tray_reset_token_error", "Errore di contatto col server Go su {1}",
            "tray_open_conf", "Apri scanner.conf",
            "tray_clear_isbn", "Svuota ISBN in sospeso",
            "tray_open_log", "Apri log debug",
            "tray_language", "Lingua / Language",
            "lang_auto", "Automatico (Rileva sistema)",
            "lang_it", "Italiano",
            "lang_en", "English",
            "tray_exit", "Esci"
        )

        this.dict["en"] := Map(
            "app_title", "ISBN Bridge",
            "lang_switched", "Language set to: English",
            "client_active_tip", "ISBN Bridge client active (Port: {1})",
            "tray_icon_tip", "Click to show QR code",
            "listener_error_title", "ISBN Bridge - Error",
            "listener_error_msg", "Unable to start AutoHotkey listener on port {1}.`n`nThe port may already be in use.",
            "qr_hint", "Scan to pair • Click or ESC to close",
            "qr_download_error", "Unable to download QR code from Go server.",
            "isbn_ready_tooltip", "ISBN ready: {1}`nClick in text field to paste.`nESC to cancel.",
            "isbn_ready_tray", "ISBN ready: {1}",
            "isbn_pasted_tooltip", "ISBN pasted: {1}",
            "tray_active", "Active",
            "tray_active_tip_on", "Auto-paste enabled.",
            "tray_active_tip_off", "Auto-paste disabled.",
            "tray_expiry", "Token Expiry",
            "tray_expiry_min", "{1} minutes",
            "tray_expiry_hour", "{1} hour ({2} min)",
            "tray_expiry_hours", "{1} hours ({2} min)",
            "tray_expiry_set_tip", "Token expiry set to {1} minutes.`nNew QR generated!",
            "tray_expiry_saved_tip", "Expiry saved to configuration file.",
            "tray_overwrite", "Overwrite text (Ctrl+A)",
            "tray_overwrite_tip_on", "Text overwrite enabled.",
            "tray_overwrite_tip_off", "Text overwrite disabled.",
            "tray_auto_hover", "Auto-paste on hover (no click)",
            "tray_auto_hover_tip_on", "Hover auto-paste enabled.",
            "tray_auto_hover_tip_off", "Hover auto-paste disabled.",
            "tray_sound", "Audio Feedback",
            "tray_sound_enable", "Enable sound",
            "tray_sound_tip_on", "Audio feedback enabled.",
            "tray_sound_tip_off", "Audio feedback disabled.",
            "sound_tap", "Tap (Soft / Default)",
            "sound_ios", "iOS Tock (Haptic tick)",
            "sound_bubble", "Bubble Pop (Soft drop)",
            "sound_chime", "Gentle Chime (Marimba tone)",
            "sound_beep", "Modern Beep (Subtle laser)",
            "sound_click", "Mechanical Click (Key switch)",
            "sound_nav", "Windows Navigation (System tick)",
            "tray_auto_qr", "Show QR code on token refresh",
            "tray_show_qr", "Show centered QR code",
            "tray_reset_token", "Reset token (New QR)",
            "tray_reset_token_success", "Token reset successfully!`nNew QR generated.",
            "tray_reset_token_error", "Error contacting Go server at {1}",
            "tray_open_conf", "Open scanner.conf",
            "tray_clear_isbn", "Clear pending ISBN",
            "tray_open_log", "Open debug log",
            "tray_language", "Language / Lingua",
            "lang_auto", "Auto (System default)",
            "lang_it", "Italiano (Italian)",
            "lang_en", "English",
            "tray_exit", "Exit"
        )
    }
}
