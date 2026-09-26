#Requires AutoHotkey v2.0

class QRModal {
    static guiInstance := 0
    static autoHideTimer := ObjBindMethod(QRModal, "Hide")

    static Show(*) {
        tempQR := A_Temp "\biblios_qr.png"
        try {
            Download(AppConfig.goServerUrl "/qr.png", tempQR)
        } catch as err {
            TrayTip("Impossibile scaricare il QR code dal server Go.", "Biblios", "Iconx")
            return
        }

        this.Hide()

        this.guiInstance := Gui("+AlwaysOnTop -Caption +Border +ToolWindow", "Biblios Token")
        this.guiInstance.BackColor := "0xFFFFFF"
        this.guiInstance.MarginX := 24
        this.guiInstance.MarginY := 20

        size := AppConfig.qrPopupSize

        this.guiInstance.SetFont("s13 bold c0f172a", "Segoe UI")
        this.guiInstance.Add("Text", "Center w" size, "Scansiona Token Biblios")

        this.guiInstance.SetFont("s9 norm c64748b", "Segoe UI")
        this.guiInstance.Add("Text", "Center w" size " y+4", "Inquadra con l'iPhone per abbinare il comando")

        imgCtrl := this.guiInstance.Add("Picture", "w" size " h" size " Center y+14", tempQR)
        imgCtrl.OnEvent("Click", (*) => this.Hide())

        this.guiInstance.SetFont("s9 bold c2563eb", "Segoe UI")
        ttlText := (AppConfig.tokenTtlMinutes >= 60)
            ? (Round(AppConfig.tokenTtlMinutes / 60, 1) " ore")
            : (AppConfig.tokenTtlMinutes " minuti")
        this.guiInstance.Add("Text", "Center w" size " y+12", "Validità token: " ttlText)

        this.guiInstance.SetFont("s8 norm c94a3b8", "Segoe UI")
        this.guiInstance.Add("Text", "Center w" size " y+4", "Fai clic sul QR o premi ESC per chiudere")

        this.guiInstance.OnEvent("Escape", (*) => this.Hide())
        this.guiInstance.OnEvent("Close", (*) => this.Hide())

        this.guiInstance.Show("Center")

        if (AppConfig.qrAutoHideSeconds > 0)
            SetTimer(this.autoHideTimer, -AppConfig.qrAutoHideSeconds * 1000)
    }

    static Hide(*) {
        SetTimer(QRModal.autoHideTimer, 0)
        if (QRModal.guiInstance) {
            QRModal.guiInstance.Destroy()
            QRModal.guiInstance := 0
        }
    }
}
