#Requires AutoHotkey v2.0

class QRModal {
    static guiInstance := 0
    static autoHideTimer := ObjBindMethod(QRModal, "Hide")

    static Show(*) {
        if (this.guiInstance) {
            ; Already on screen: re-activate and extend auto-hide
            ; instead of rebuilding (avoids flicker on repeat clicks).
            try WinActivate(this.guiInstance.Hwnd)
            if (AppConfig.qrAutoHideSeconds > 0)
                SetTimer(this.autoHideTimer, -AppConfig.qrAutoHideSeconds * 1000)
            return
        }

        Logger.Log("QRModal.Show invoked")
        tempQR := A_Temp "\isbn_bridge_qr_" Random(100000, 999999) "_" A_TickCount ".png"
        if !this.TryDownload(tempQR) {
            ; Server may be down: (re)start it, then retry once before
            ; giving up with an error notification.
            if ServerManager.EnsureRunning() && this.TryDownload(tempQR, 400)
                Logger.Log("QRModal download succeeded after server restart")
            else {
                TrayTip(I18n.Get("qr_download_error"), I18n.Get("app_title"), "Iconx")
                return
            }
        }

        ; Ultra-minimal, borderless floating QR card
        this.guiInstance := Gui("+AlwaysOnTop -Caption +Border +ToolWindow", I18n.Get("app_title"))
        this.guiInstance.BackColor := "0xFFFFFF"
        this.guiInstance.MarginX := 16
        this.guiInstance.MarginY := 6

        size := AppConfig.qrPopupSize

        ; Brand artwork, padded sides: all gaps controlled here
        brandPath := A_ScriptDir "\assets\brand.png"
        if FileExist(brandPath) {
            ; Artwork is 516x55, displayed at QR width
            brandCtrl := this.guiInstance.Add("Picture", "w" size " h" (size * 55 // 516) " Center y+2", brandPath)
            brandCtrl.OnEvent("Click", (*) => this.Hide())
        }

        ; Crisp QR Code
        imgCtrl := this.guiInstance.Add("Picture", "w" size " h" size " Center y+2", tempQR)
        imgCtrl.OnEvent("Click", (*) => this.Hide())

        ; The PNG embeds the live pairing token: remove it once the
        ; control has loaded it so nothing token-bearing lingers in %TEMP%.
        try FileDelete(tempQR)
        catch as err {
            Logger.Log("QRModal temp cleanup failed: " err.Message)
        }

        ; Minimal single-line hint, matched to brand ink
        this.guiInstance.SetFont("s8 norm c2F4A6E", "Tahoma")
        hintCtrl := this.guiInstance.Add("Text", "Center w" size " y+8", I18n.Get("qr_hint"))
        hintCtrl.OnEvent("Click", (*) => this.Hide())

        this.guiInstance.OnEvent("Escape", (*) => this.Hide())
        this.guiInstance.OnEvent("Close", (*) => this.Hide())

        this.guiInstance.Show("Center")
        WinActivate(this.guiInstance.Hwnd)
        Logger.Log("QRModal shown successfully (HWND: " this.guiInstance.Hwnd ")")

        if (AppConfig.qrAutoHideSeconds > 0)
            SetTimer(this.autoHideTimer, -AppConfig.qrAutoHideSeconds * 1000)
    }

    static TryDownload(tempQR, delay := 0) {
        if delay
            Sleep(delay)
        try {
            Download(AppConfig.goServerUrl "/qr.png", tempQR)
            return true
        } catch as err {
            Logger.Log("QRModal download failed: " err.Message)
            return false
        }
    }

    static Hide(*) {
        SetTimer(QRModal.autoHideTimer, 0)
        if (QRModal.guiInstance) {
            Logger.Log("QRModal hidden")
            QRModal.guiInstance.Destroy()
            QRModal.guiInstance := 0
        }
    }
}
