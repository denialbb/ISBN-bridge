#Requires AutoHotkey v2.0

class ServerManager {
    static serverPid := 0
    static isManaged := false
    static isConsoleShown := false

    static StartOrAttach() {
        if this.IsRunning() {
            ; Server is already running. Ensure console matches configuration.
            if AppConfig.hideConsole
                this.HideConsole()
            return true
        }

        ; Server is not running. Launch it automatically in background.
        binPath := this.FindBinary()
        if !binPath {
            Logger.Log("Warning: isbn-bridge binary not found. Start it manually.")
            return false
        }

        ; Compiled release layout is flat (exe + assets side by side),
        ; so the working dir is the exe dir itself.
        workDir := A_IsCompiled ? A_ScriptDir : A_ScriptDir "\.."

        try {
            Run('"' binPath '"', workDir, "Hide", &managedPid)
            this.serverPid := managedPid
            this.isManaged := true
            Logger.Log("Launched Go server (PID: " managedPid ") in background")

            ; Give process a moment to initialize
            Sleep(400)
            if AppConfig.hideConsole
                this.HideConsole()
            return true
        } catch as err {
            Logger.Log("Failed to launch Go server: " err.Message)
            return false
        }
    }

    static FindBinary() {
        candidates := [
            A_ScriptDir "\isbn-bridge.exe",
            A_ScriptDir "\..\bin\isbn-bridge.exe",
            A_ScriptDir "\bin\isbn-bridge.exe",
            A_WorkingDir "\bin\isbn-bridge.exe"
        ]
        for path in candidates {
            if FileExist(path)
                return path
        }
        return ""
    }

    static IsRunning() {
        return ProcessExist("isbn-bridge.exe") != 0
    }

    ; Make sure the Go server answers. Starts it if needed and polls
    ; /health briefly. Used before actions that require the server so a
    ; dead server restarts transparently instead of erroring.
    static EnsureRunning() {
        if this.IsRunning()
            return true
        if !this.StartOrAttach()
            return false
        Loop 20 {
            Sleep(200)
            try {
                req := ComObject("MSXML2.XMLHTTP")
                req.open("GET", AppConfig.goServerUrl "/health", false)
                req.send()
                if (req.status = 200)
                    return true
            } catch {
            }
        }
        return this.IsRunning()
    }

    static GetWindow() {
        DetectHiddenWindows(true)
        if this.serverPid && (hwnd := WinExist("ahk_pid " this.serverPid))
            return hwnd
        if hwnd := WinExist("ISBN Bridge Server ahk_class ConsoleWindowClass")
            return hwnd
        if hwnd := WinExist("ISBN Bridge Server")
            return hwnd
        if hwnd := WinExist("ahk_exe isbn-bridge.exe")
            return hwnd
        return 0
    }

    static IsConsoleVisible() {
        DetectHiddenWindows(true)
        hwnd := this.GetWindow()
        if !hwnd
            return false
        style := WinGetStyle(hwnd)
        return (style & 0x10000000) != 0 ; WS_VISIBLE bit
    }

    static ShowConsole() {
        DetectHiddenWindows(true)
        hwnd := this.GetWindow()
        if hwnd {
            WinShow(hwnd)
            WinActivate(hwnd)
            this.isConsoleShown := true
        }

        ; Call Go HTTP console endpoint as fallback/sync
        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/console/show", false)
            req.send()
        }
        return true
    }

    static HideConsole() {
        DetectHiddenWindows(true)
        hwnd := this.GetWindow()
        if hwnd {
            WinHide(hwnd)
            this.isConsoleShown := false
        }

        ; Call Go HTTP console endpoint as fallback/sync
        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/console/hide", false)
            req.send()
        }
        return true
    }

    static ToggleConsole() {
        if this.IsConsoleVisible() {
            this.HideConsole()
            TrayTip(I18n.Get("server_console_hidden"), I18n.Get("app_title"))
        } else {
            this.ShowConsole()
            TrayTip(I18n.Get("server_console_shown"), I18n.Get("app_title"))
        }
        TrayManager.UpdateServerMenu()
    }

    static Shutdown() {
        if !this.isManaged
            return

        Logger.Log("Shutting down managed Go server...")

        ; 1. Request graceful HTTP shutdown
        try {
            req := ComObject("MSXML2.XMLHTTP")
            req.open("POST", AppConfig.goServerUrl "/shutdown", false)
            req.send()
        }

        Sleep(200)

        ; 2. Terminate server if still running
        if this.serverPid && ProcessExist(this.serverPid)
            ProcessClose(this.serverPid)
        if ProcessExist("isbn-bridge.exe")
            ProcessClose("isbn-bridge.exe")
    }
}
