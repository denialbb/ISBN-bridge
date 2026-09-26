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
        FollowToolTip.Show(I18n.Get("isbn_ready_tooltip", isbn))
        TrayTip(I18n.Get("isbn_ready_tray", isbn), I18n.Get("app_title"))
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

        ; Re-validate the target window: it may have changed during the
        ; deferred-click delay, and we must never paste into the wrong app.
        if !this.IsTargetWindowActive()
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

            ; Final guard right before keystrokes go out: the focused
            ; window must be the target and the cursor must be an IBeam.
            ; Checked after the focus click so hover-paste into a
            ; background window still works, while a window switch in
            ; the arming gap aborts instead of mistyping elsewhere.
            if !this.IsTargetWindowActive() || A_Cursor != "IBeam" {
                Logger.Log("PasteNow aborted: target window/cursor check failed for ISBN " isbn)
                return
            }

            A_Clipboard := isbn
            Sleep(30)

            if AppConfig.overwriteExistingText {
                SendInput("^a")
                Sleep(25)
            }
            SendInput("^v")

            SoundManager.PlayTap()
            FollowToolTip.Flash(I18n.Get("isbn_pasted_tooltip", isbn))

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

    ; Active-window counterpart of the hover check: the currently focused
    ; window must match the configured target tab title (empty filter
    ; means any active window is acceptable).
    static IsTargetWindowActive() {
        if (AppConfig.targetTabTitle = "")
            return true

        title := ""
        try title := WinGetTitle("A")
        if (title = "")
            return false

        return InStr(title, AppConfig.targetTabTitle, false) > 0
    }

    static Cancel(*) {
        PasteEngine.pendingISBN := ""
        FollowToolTip.Hide()
    }
}
