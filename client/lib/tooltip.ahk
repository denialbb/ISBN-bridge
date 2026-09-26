#Requires AutoHotkey v2.0

class FollowToolTip {
    static hwnd := 0
    static lastX := -1
    static lastY := -1
    static currentText := ""
    static updateTimer := ObjBindMethod(FollowToolTip, "Update")
    static flashTimer := ObjBindMethod(FollowToolTip, "Hide")

    static Show(text) {
        this.Hide()
        this.currentText := text
        MouseGetPos(&mx, &my)
        this.lastX := mx
        this.lastY := my

        ToolTip(
            text,
            mx + AppConfig.tooltipOffsetX,
            my + AppConfig.tooltipOffsetY
        )
        this.hwnd := WinExist("ahk_class tooltips_class32 ahk_pid " ProcessExist())
        SetTimer(this.updateTimer, 33)
    }

    static Update() {
        if (!this.hwnd && this.currentText = "") {
            SetTimer(this.updateTimer, 0)
            return
        }

        MouseGetPos(&mx, &my)
        if (mx = this.lastX && my = this.lastY)
            return

        this.lastX := mx
        this.lastY := my

        if this.hwnd {
            ; SWP_NOSIZE (0x0001) | SWP_NOZORDER (0x0004) | SWP_NOACTIVATE (0x0010) = 0x0015
            DllCall(
                "User32.dll\SetWindowPos",
                "Ptr", this.hwnd,
                "Ptr", 0,
                "Int", mx + AppConfig.tooltipOffsetX,
                "Int", my + AppConfig.tooltipOffsetY,
                "Int", 0,
                "Int", 0,
                "UInt", 0x0015
            )
        } else {
            ToolTip(this.currentText, mx + AppConfig.tooltipOffsetX, my + AppConfig.tooltipOffsetY)
        }
    }

    static Flash(text, durationMs := 1200) {
        this.Hide()
        this.currentText := text
        MouseGetPos(&mx, &my)
        ToolTip(text, mx + AppConfig.tooltipOffsetX, my + AppConfig.tooltipOffsetY)
        SetTimer(this.flashTimer, -durationMs)
    }

    static Hide(*) {
        SetTimer(FollowToolTip.updateTimer, 0)
        SetTimer(FollowToolTip.flashTimer, 0)
        FollowToolTip.hwnd := 0
        FollowToolTip.lastX := -1
        FollowToolTip.lastY := -1
        FollowToolTip.currentText := ""
        ToolTip()
    }
}
