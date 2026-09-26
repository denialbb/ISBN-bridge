#Requires AutoHotkey v2.0

class QRModal {
    static guiInstance := 0
    static autoHideTimer := ObjBindMethod(QRModal, "Hide")

    static Show(*) {
        Logger.Log("QRModal.Show invoked")
        tempQR := A_Temp "\isbn_bridge_qr.png"
        try {
            Download(AppConfig.goServerUrl "/qr.png", tempQR)
        } catch as err {
            Logger.Log("QRModal download failed: " err.Message)
            TrayTip(I18n.Get("qr_download_error"), I18n.Get("app_title"), "Iconx")
            return
        }

        this.Hide()

        ; Ultra-minimal, borderless floating QR card
        this.guiInstance := Gui("+AlwaysOnTop -Caption +Border +ToolWindow", I18n.Get("app_title"))
        this.guiInstance.BackColor := "0xFFFFFF"
        this.guiInstance.MarginX := 6
        this.guiInstance.MarginY := 6

        size := AppConfig.qrPopupSize

        ; Crisp QR Code
        imgCtrl := this.guiInstance.Add("Picture", "w" size " h" size " Center", tempQR)
        imgCtrl.OnEvent("Click", (*) => this.Hide())

        ; Minimal single-line hint
        this.guiInstance.SetFont("s8 norm c64748b", "Segoe UI")
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

    static Hide(*) {
        SetTimer(QRModal.autoHideTimer, 0)
        if (QRModal.guiInstance) {
            Logger.Log("QRModal hidden")
            QRModal.guiInstance.Destroy()
            QRModal.guiInstance := 0
        }
    }
}
