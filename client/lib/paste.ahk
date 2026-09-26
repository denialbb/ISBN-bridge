#Requires AutoHotkey v2.0

class PasteEngine {
    static pendingISBN := ""
    static suppressClipboard := false
    static enabled := true

    static Arm(isbn) {
        if !this.enabled
            return

        ; If mouse is already hovering over an input field in the target window,
        ; insert automatically without requiring a click
        if AppConfig.autoPasteOnHover && this.IsMouseOverTargetField() {
            this.PasteNow(isbn, true)
            return
        }

        this.pendingISBN := isbn
        FollowToolTip.Show("ISBN pronto: " isbn "`nClicca nel campo di testo per incollarlo.`nESC per annullare.")
        TrayTip("ISBN pronto: " isbn, "ISBN Bridge")
    }

    static HandleLeftClick() {
        if (!this.enabled || this.pendingISBN = "")
            return

        MouseGetPos(, , &windowID)
        if !windowID
            return

        title := ""
        try title := WinGetTitle("ahk_id " windowID)

        if (AppConfig.targetTabTitle != "") && !InStr(title, AppConfig.targetTabTitle, false)
            return

        wasIBeam := (A_Cursor = "IBeam")
        isbn := this.pendingISBN

        ; Allow the browser to process the click and focus the input field first
        SetTimer(() => this.OnDeferredClick(isbn, wasIBeam), -100)
    }

    static OnDeferredClick(isbn, wasIBeam) {
        if (this.pendingISBN != isbn)
            return

        if (!wasIBeam && A_Cursor != "IBeam")
            return

        this.PasteNow(isbn, false)
    }

    static PasteNow(isbn, autoFocusWithClick := false) {
        this.suppressClipboard := true
        FollowToolTip.Hide()
        this.pendingISBN := ""

        try {
            if autoFocusWithClick {
                Click()
                Sleep(40)
            }

            A_Clipboard := isbn
            Sleep(30)

            if AppConfig.overwriteExistingText {
                SendInput("^a")
                Sleep(25)
            }
            SendInput("^v")

            SoundManager.PlayTap()
            FollowToolTip.Flash("ISBN incollato: " isbn)

        } finally {
            SetTimer(() => (PasteEngine.suppressClipboard := false), -300)
        }
    }

    static IsMouseOverTargetField() {
        if (A_Cursor != "IBeam")
            return false

        MouseGetPos(, , &windowID)
        if !windowID
            return false

        title := ""
        try title := WinGetTitle("ahk_id " windowID)

        if (AppConfig.targetTabTitle = "")
            return true

        return InStr(title, AppConfig.targetTabTitle, false) > 0
    }

    static Cancel(*) {
        PasteEngine.pendingISBN := ""
        FollowToolTip.Hide()
    }
}
