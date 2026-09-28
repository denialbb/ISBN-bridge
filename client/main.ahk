#Requires AutoHotkey v2.0
#SingleInstance Force
Persistent(true)

CoordMode("ToolTip", "Screen")
CoordMode("Mouse", "Screen")

#Include "lib/config.ahk"
#Include "lib/i18n.ahk"
#Include "lib/logger.ahk"
#Include "lib/theme.ahk"

; Catch and log any unhandled runtime exceptions
OnError((err, mode) => (Logger.Log("UNHANDLED ERROR: " err.Message " at line " err.Line " in " err.File), 0))

#Include "lib/sound.ahk"
#Include "lib/tooltip.ahk"
#Include "lib/ui_qr.ahk"
#Include "lib/isbn.ahk"
#Include "lib/paste.ahk"
#Include "lib/server.ahk"
#Include "lib/server_manager.ahk"
#Include "lib/tray.ahk"

; Initialize configuration from scanner.conf
AppConfig.Init()
I18n.Init()

; Register cleanup on shutdown
OnExit((*) => (HttpListener.Shutdown(), ServerManager.Shutdown()))

; Start local HTTP listener
if !HttpListener.Start(AppConfig.httpPort) {
    MsgBox(
        I18n.Get("listener_error_msg", AppConfig.httpPort),
        I18n.Get("listener_error_title"),
        "Iconx"
    )
    ExitApp()
}

; Start or attach to Go server (hidden console)
ServerManager.StartOrAttach()

; Build system tray menu
TrayManager.Init()

; Register mouse & keyboard hotkeys
Hotkey("~LButton", (*) => PasteEngine.HandleLeftClick())
Hotkey("~Esc", (*) => (PasteEngine.Cancel(), QRModal.Hide()))

TrayTip(I18n.Get("client_active_tip", AppConfig.httpPort), I18n.Get("app_title"))

; First-run hint: teach left-click-to-QR once, then remember via flag file
try {
    flagDir := A_AppData "\ISBN Bridge"
    flagFile := flagDir "\seen-hint.flag"
    if !FileExist(flagFile) {
        DirCreate(flagDir)
        FileAppend("seen", flagFile, "UTF-8")
        TrayTip(I18n.Get("tray_firstrun_hint"), I18n.Get("app_title"))
    }
}

; Show QR modal on startup if configured
if AppConfig.qrAutoShowOnRefresh
    SetTimer(() => QRModal.Show(), -400)
