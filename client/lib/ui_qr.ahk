#Requires AutoHotkey v2.0

class QRModal {
    static guiInstance := 0
    static autoHideTimer := ObjBindMethod(QRModal, "Hide")

    static Show(*) {
        Logger.Log("QRModal.Show invoked")
        tempQR := A_Temp "\isbn_bridge_qr.png"
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

        this.Hide()

        ; Ultra-minimal, borderless floating QR card
        this.guiInstance := Gui("+AlwaysOnTop -Caption +Border +ToolWindow", I18n.Get("app_title"))
        this.guiInstance.BackColor := "0xFFFFFF"
        this.guiInstance.MarginX := 16
        this.guiInstance.MarginY := 8

        size := AppConfig.qrPopupSize

        ; Brand line on top: bundled Fira Code, faded slate
        this.guiInstance.SetFont("s12 bold c7d8ca3", "Fira Code")
        brandCtrl := this.guiInstance.Add("Text", "Center w" size, "ISBN Bridge")
        brandCtrl.OnEvent("Click", (*) => this.Hide())

        ; Crisp QR Code
        imgCtrl := this.guiInstance.Add("Picture", "w" size " h" size " Center y+6", tempQR)
        imgCtrl.OnEvent("Click", (*) => this.Hide())

        ; Minimal single-line hint in Tahoma
        this.guiInstance.SetFont("s8 norm c64748b", "Tahoma")
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
