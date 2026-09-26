#Requires AutoHotkey v2.0
#SingleInstance Force
Persistent(true)

CoordMode("ToolTip", "Screen")
CoordMode("Mouse", "Screen")

#Include "lib/config.ahk"
#Include "lib/logger.ahk"
#Include "lib/sound.ahk"
#Include "lib/tooltip.ahk"
#Include "lib/ui_qr.ahk"
#Include "lib/paste.ahk"
#Include "lib/server.ahk"
#Include "lib/tray.ahk"

; Initialize configuration from scanner.conf
AppConfig.Init()

; Register cleanup on shutdown
OnExit((*) => HttpListener.Shutdown())

; Start local HTTP listener
if !HttpListener.Start(AppConfig.httpPort) {
    MsgBox(
        "Impossibile avviare il listener AutoHotkey sulla porta " AppConfig.httpPort ".`n`n"
        "La porta potrebbe essere già in uso.",
        "Biblios",
        "Iconx"
    )
    ExitApp()
}

; Build system tray menu
TrayManager.Init()

; Register mouse & keyboard hotkeys
Hotkey("~LButton", (*) => PasteEngine.HandleLeftClick())
Hotkey("~Esc", (*) => (PasteEngine.Cancel(), QRModal.Hide()))

TrayTip("Client Biblios attivo (Porta: " AppConfig.httpPort ")", "Biblios")
