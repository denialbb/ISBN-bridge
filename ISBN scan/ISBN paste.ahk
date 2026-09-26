#Requires AutoHotkey v2.0
#SingleInstance Force
Persistent

CoordMode("ToolTip", "Screen")
CoordMode("Mouse", "Screen")

; ============================================================
; Biblios ISBN Auto-Paste Client
; ============================================================

; ----------------------------
; Configuration loading (scanner.conf)
; ----------------------------

global CONF_FILE := FindConfigFile()

FindConfigFile() {
    if FileExist(A_ScriptDir "\scanner.conf")
        return A_ScriptDir "\scanner.conf"
    if FileExist(A_ScriptDir "\..\scanner.conf")
        return A_ScriptDir "\..\scanner.conf"
    return A_ScriptDir "\scanner.conf"
}

; Server & Connection settings
global HTTP_PORT := 8766
global GO_PORT := 8765
global GO_SERVER_URL := "http://127.0.0.1:8765"
global tokenTTLMinutes := 60

; AutoPaste settings
global TARGET_TAB_TITLE := "hardcover"
global overwriteExistingText := true
global autoPasteOnHover := true
global playTapSoundEnabled := true
global TOOLTIP_OFFSET_X := 10
global TOOLTIP_OFFSET_Y := 12

; QR Code settings
global qrAutoShowOnRefresh := true
global qrAutoHideSeconds := 45
global qrPopupSize := 280

LoadConfig()

LoadConfig() {
    global CONF_FILE
    global HTTP_PORT
    global GO_PORT
    global GO_SERVER_URL
    global tokenTTLMinutes
    global TARGET_TAB_TITLE
    global overwriteExistingText
    global autoPasteOnHover
    global playTapSoundEnabled
    global TOOLTIP_OFFSET_X
    global TOOLTIP_OFFSET_Y
    global qrAutoShowOnRefresh
    global qrAutoHideSeconds
    global qrPopupSize

    HTTP_PORT := Integer(IniRead(CONF_FILE, "Server", "ahk_port", 8766))
    GO_PORT := Integer(IniRead(CONF_FILE, "Server", "port", 8765))
    GO_SERVER_URL := "http://127.0.0.1:" GO_PORT
    tokenTTLMinutes := Integer(IniRead(CONF_FILE, "Server", "token_ttl_minutes", 60))

    TARGET_TAB_TITLE := IniRead(CONF_FILE, "AutoPaste", "target_tab_title", "hardcover")
    overwriteExistingText := (IniRead(CONF_FILE, "AutoPaste", "overwrite_existing_text", "true") = "true")
    autoPasteOnHover := (IniRead(CONF_FILE, "AutoPaste", "auto_paste_on_hover", "true") = "true")
    playTapSoundEnabled := (IniRead(CONF_FILE, "AutoPaste", "play_tap_sound", "true") = "true")

    TOOLTIP_OFFSET_X := Integer(IniRead(CONF_FILE, "AutoPaste", "tooltip_offset_x", 10))
    TOOLTIP_OFFSET_Y := Integer(IniRead(CONF_FILE, "AutoPaste", "tooltip_offset_y", 12))

    qrAutoShowOnRefresh := (IniRead(CONF_FILE, "QRCode", "auto_show_on_refresh", "true") = "true")
    qrAutoHideSeconds := Integer(IniRead(CONF_FILE, "QRCode", "auto_hide_seconds", 45))
    qrPopupSize := Integer(IniRead(CONF_FILE, "QRCode", "popup_size", 280))
}

; Token storage (for legacy direct connections)
global TOKEN_FILE := A_ScriptDir "\token.txt"
global BIBLIOS_TOKEN := "XJHNiW2BsqPCyAl7h4TxZwLrMme9Yt3EUdK8bgO1zkQFc6Da"
if FileExist(TOKEN_FILE) {
    try {
        fileToken := Trim(FileRead(TOKEN_FILE, "UTF-8"))
        if (fileToken != "")
            BIBLIOS_TOKEN := fileToken
    }
}

global DEBUG_LOG := A_ScriptDir "\biblios-debug.log"

; ----------------------------
; Runtime state
; ----------------------------

global enabled := true
global pendingISBN := ""
global suppressClipboard := false

global tipHwnd := 0
global lastX := -1
global lastY := -1

global qrGui := 0

global serverSocket := 0
global clientStates := Map()
global winsockStarted := false
global expiryMenu := Menu()


; ============================================================
; Startup
; ============================================================

OnExit(Shutdown)

if !StartWinsock() {
    MsgBox(
        "Impossibile inizializzare Winsock.",
        "Biblios",
        "Iconx"
    )
    ExitApp
}

if !StartHttpServer() {
    MsgBox(
        "Impossibile avviare il listener AutoHotkey sulla porta "
        HTTP_PORT ".`n`n"
        "La porta potrebbe essere già in uso.",
        "Biblios",
        "Iconx"
    )
    ExitApp
}

OnClipboardChange(ClipChanged)

; Poll non-blocking sockets
SetTimer(PollSockets, 25)

; Check mouse clicks
Hotkey("~LButton", HandleLeftClick)

; ESC cancels pending ISBN and dismisses QR
Hotkey("~Esc", HandleEscapeKey)

CreateTrayMenu()

TrayTip(
    "Client Biblios attivo (Porta: " HTTP_PORT ")",
    "Biblios"
)


; ============================================================
; Tray menu & User Options
; ============================================================

CreateTrayMenu() {
    global expiryMenu

    A_TrayMenu.Delete()

    A_TrayMenu.Add("Attivo", ToggleEnabled)
    A_TrayMenu.Add()

    ; --- Submenu: Expiry Duration ---
    expiryMenu := Menu()
    expiryMenu.Add("15 minuti", (*) => SetExpiryOption(15))
    expiryMenu.Add("30 minuti", (*) => SetExpiryOption(30))
    expiryMenu.Add("1 ora (60 min)", (*) => SetExpiryOption(60))
    expiryMenu.Add("2 ore (120 min)", (*) => SetExpiryOption(120))
    expiryMenu.Add("12 ore (720 min)", (*) => SetExpiryOption(720))
    expiryMenu.Add("24 ore (1440 min)", (*) => SetExpiryOption(1440))

    A_TrayMenu.Add("Scadenza token", expiryMenu)
    A_TrayMenu.Add()

    ; --- Options ---
    A_TrayMenu.Add("Sovrascrivi testo (Ctrl+A)", ToggleOverwrite)
    A_TrayMenu.Add("Auto-incolla al passaggio (senza clic)", ToggleAutoHover)
    A_TrayMenu.Add("Suono al tocco (Tap)", ToggleSound)
    A_TrayMenu.Add("Mostra QR code al cambio token", ToggleAutoQR)
    A_TrayMenu.Add()

    ; --- Actions ---
    A_TrayMenu.Add("Mostra QR code al centro", ShowCenteredQR)
    A_TrayMenu.Add("Reset token (Nuovo QR)", ResetToken)
    A_TrayMenu.Add("Apri scanner.conf", (*) => Run(CONF_FILE))
    A_TrayMenu.Add("Svuota ISBN in sospeso", CancelISBN)
    A_TrayMenu.Add("Apri log debug", OpenDebugLog)
    A_TrayMenu.Add()

    A_TrayMenu.Add("Esci", (*) => ExitApp())

    UpdateTrayState()
}


ToggleEnabled(*) {
    global enabled

    enabled := !enabled
    UpdateTrayState()

    if enabled {
        TrayTip("Auto-incolla attivo.", "Biblios")
    } else {
        CancelISBN()
        TrayTip("Auto-incolla disattivato.", "Biblios")
    }
}


ToggleOverwrite(*) {
    global overwriteExistingText
    global CONF_FILE

    overwriteExistingText := !overwriteExistingText
    IniWrite(overwriteExistingText ? "true" : "false", CONF_FILE, "AutoPaste", "overwrite_existing_text")
    UpdateTrayState()

    TrayTip(
        overwriteExistingText ? "Sovrascrittura testo attiva (Ctrl+A)." : "Sovrascrittura testo disattivata.",
        "Biblios"
    )
}


ToggleAutoHover(*) {
    global autoPasteOnHover
    global CONF_FILE

    autoPasteOnHover := !autoPasteOnHover
    IniWrite(autoPasteOnHover ? "true" : "false", CONF_FILE, "AutoPaste", "auto_paste_on_hover")
    UpdateTrayState()

    TrayTip(
        autoPasteOnHover ? "Auto-incolla al passaggio attivo (senza clic)." : "Auto-incolla al passaggio disattivato.",
        "Biblios"
    )
}


ToggleSound(*) {
    global playTapSoundEnabled
    global CONF_FILE

    playTapSoundEnabled := !playTapSoundEnabled
    IniWrite(playTapSoundEnabled ? "true" : "false", CONF_FILE, "AutoPaste", "play_tap_sound")
    UpdateTrayState()

    TrayTip(
        playTapSoundEnabled ? "Suono al tocco attivo." : "Suono al tocco disattivato.",
        "Biblios"
    )
}


ToggleAutoQR(*) {
    global qrAutoShowOnRefresh
    global CONF_FILE

    qrAutoShowOnRefresh := !qrAutoShowOnRefresh
    IniWrite(qrAutoShowOnRefresh ? "true" : "false", CONF_FILE, "QRCode", "auto_show_on_refresh")
    UpdateTrayState()
}


SetExpiryOption(minutes) {
    global tokenTTLMinutes
    global CONF_FILE
    global GO_SERVER_URL

    tokenTTLMinutes := minutes
    IniWrite(String(minutes), CONF_FILE, "Server", "token_ttl_minutes")
    UpdateTrayState()

    try {
        req := ComObject("MSXML2.XMLHTTP")
        req.open("POST", GO_SERVER_URL "/token/ttl?minutes=" minutes, false)
        req.send()

        TrayTip("Scadenza token aggiornata a " minutes " minuti!`nNuovo QR code generato.", "Biblios")
        ShowCenteredQR()
    } catch as err {
        TrayTip("Scadenza salvata nel file di configurazione.", "Biblios")
    }
}


ResetToken(*) {
    global GO_SERVER_URL

    try {
        req := ComObject("MSXML2.XMLHTTP")
        req.open("POST", GO_SERVER_URL "/token/refresh", false)
        req.setRequestHeader("Accept", "application/json")
        req.send()

        TrayTip("Token reimpostato con successo!`nNuovo QR generato.", "Biblios")
        ShowCenteredQR()
    } catch as err {
        tokenScript := A_ScriptDir "\Generate Token.ahk"
        if FileExist(tokenScript) {
            Run('"' A_AhkPath '" "' tokenScript '"')
        } else {
            MsgBox(
                "Impossibile contattare il server Go su " GO_SERVER_URL ":`n" err.Message,
                "Biblios",
                "Iconx"
            )
        }
    }
}


UpdateTrayState() {
    global enabled
    global overwriteExistingText
    global autoPasteOnHover
    global playTapSoundEnabled
    global qrAutoShowOnRefresh
    global tokenTTLMinutes
    global expiryMenu

    A_TrayMenu.Uncheck("Attivo")
    if enabled
        A_TrayMenu.Check("Attivo")

    A_TrayMenu.Uncheck("Sovrascrivi testo (Ctrl+A)")
    if overwriteExistingText
        A_TrayMenu.Check("Sovrascrivi testo (Ctrl+A)")

    A_TrayMenu.Uncheck("Auto-incolla al passaggio (senza clic)")
    if autoPasteOnHover
        A_TrayMenu.Check("Auto-incolla al passaggio (senza clic)")

    A_TrayMenu.Uncheck("Suono al tocco (Tap)")
    if playTapSoundEnabled
        A_TrayMenu.Check("Suono al tocco (Tap)")

    A_TrayMenu.Uncheck("Mostra QR code al cambio token")
    if qrAutoShowOnRefresh
        A_TrayMenu.Check("Mostra QR code al cambio token")

    ; Update Expiry Submenu checks
    expiryOptions := [15, 30, 60, 120, 720, 1440]
    expiryLabels := ["15 minuti", "30 minuti", "1 ora (60 min)", "2 ore (120 min)", "12 ore (720 min)", "24 ore (1440 min)"]

    Loop expiryOptions.Length {
        opt := expiryOptions[A_Index]
        label := expiryLabels[A_Index]
        expiryMenu.Uncheck(label)
        if (tokenTTLMinutes = opt)
            expiryMenu.Check(label)
    }
}


OpenDebugLog(*) {
    global DEBUG_LOG
    if !FileExist(DEBUG_LOG)
        FileAppend("", DEBUG_LOG, "UTF-8")
    Run(DEBUG_LOG)
}


HandleEscapeKey(*) {
    CancelISBN()
    HideCenteredQR()
}


; ============================================================
; Centered Seamless QR Code Overlay
; ============================================================

ShowCenteredQR(*) {
    global qrGui
    global GO_SERVER_URL
    global qrPopupSize
    global qrAutoHideSeconds
    global tokenTTLMinutes

    tempQR := A_Temp "\biblios_qr.png"
    try {
        Download(GO_SERVER_URL "/qr.png", tempQR)
    } catch as err {
        TrayTip("Impossibile scaricare il QR code dal server Go.", "Biblios", "Iconx")
        return
    }

    HideCenteredQR()

    qrGui := Gui("+AlwaysOnTop -Caption +Border +ToolWindow", "Biblios Token")
    qrGui.BackColor := "0xFFFFFF"
    qrGui.MarginX := 24
    qrGui.MarginY := 20

    ; Header
    qrGui.SetFont("s13 bold c0f172a", "Segoe UI")
    qrGui.Add("Text", "Center w" qrPopupSize, "Scansiona Token Biblios")

    qrGui.SetFont("s9 norm c64748b", "Segoe UI")
    qrGui.Add("Text", "Center w" qrPopupSize " y+4", "Inquadra con l'iPhone per abbinare il comando")

    ; QR Image in center
    imgCtrl := qrGui.Add("Picture", "w" qrPopupSize " h" qrPopupSize " Center y+14", tempQR)
    imgCtrl.OnEvent("Click", (*) => HideCenteredQR())

    ; Expiry badge
    qrGui.SetFont("s9 bold c2563eb", "Segoe UI")
    ttlText := (tokenTTLMinutes >= 60) ? (Round(tokenTTLMinutes / 60, 1) " ore") : (tokenTTLMinutes " minuti")
    qrGui.Add("Text", "Center w" qrPopupSize " y+12", "Validità token: " ttlText)

    ; Subtle hint
    qrGui.SetFont("s8 norm c94a3b8", "Segoe UI")
    qrGui.Add("Text", "Center w" qrPopupSize " y+4", "Fai clic sul QR o premi ESC per chiudere")

    qrGui.OnEvent("Escape", (*) => HideCenteredQR())
    qrGui.OnEvent("Close", (*) => HideCenteredQR())

    qrGui.Show("Center")

    ; Auto-dismiss timer
    if (qrAutoHideSeconds > 0)
        SetTimer(HideCenteredQR, -qrAutoHideSeconds * 1000)
}

HideCenteredQR(*) {
    global qrGui
    SetTimer(HideCenteredQR, 0)
    if qrGui {
        qrGui.Destroy()
        qrGui := 0
    }
}


; ============================================================
; Clipboard / ISBN
; ============================================================

ClipChanged(DataType) {
    global enabled
    global suppressClipboard

    if !enabled
        return

    if suppressClipboard
        return

    if (DataType != 1)
        return

    isbn := NormalizeISBN(A_Clipboard)

    if IsISBNFormatValid(isbn)
        ArmISBN(isbn)
}


NormalizeISBN(value) {
    value := Trim(value)
    value := RegExReplace(value, "[\s-]", "")
    return StrUpper(value)
}


IsISBNFormatValid(isbn) {
    ; ISBN-13
    if RegExMatch(isbn, "^\d{13}$") {
        sum := 0
        Loop 13 {
            digit := Integer(SubStr(isbn, A_Index, 1))
            if Mod(A_Index, 2)
                sum += digit
            else
                sum += digit * 3
        }
        return Mod(sum, 10) = 0
    }

    ; ISBN-10
    if RegExMatch(isbn, "^\d{9}[\dXx]$") {
        sum := 0
        Loop 9 {
            digit := Integer(SubStr(isbn, A_Index, 1))
            sum += digit * (11 - A_Index)
        }
        check := SubStr(isbn, 10, 1)
        if (check = "X" || check = "x")
            checkValue := 10
        else
            checkValue := Integer(check)
        sum += checkValue
        return Mod(sum, 11) = 0
    }

    return false
}


ArmISBN(isbn) {
    global pendingISBN
    global tipHwnd
    global lastX
    global lastY
    global TARGET_TAB_TITLE
    global autoPasteOnHover
    global TOOLTIP_OFFSET_X
    global TOOLTIP_OFFSET_Y

    ; Feature: If mouse is already hovering over an input field in the target window,
    ; automatically insert the ISBN without requiring a click!
    if autoPasteOnHover {
        MouseGetPos(&mx, &my, &windowID)
        title := ""
        if windowID {
            try title := WinGetTitle("ahk_id " windowID)
        }

        isTargetWindow := (TARGET_TAB_TITLE = "") || InStr(title, TARGET_TAB_TITLE, false)
        isHoveringText := (A_Cursor = "IBeam")

        if (isTargetWindow && isHoveringText) {
            AutoPasteHovered(isbn)
            return
        }
    }

    ; Otherwise, arm for manual click
    pendingISBN := isbn
    SetTimer(ClearToolTip, 0)

    lastX := mx
    lastY := my

    tipHwnd := ToolTip(
        "ISBN pronto: " isbn "`n"
        "Clicca nel campo di testo per incollarlo.`n"
        "ESC per annullare.",
        mx + TOOLTIP_OFFSET_X,
        my + TOOLTIP_OFFSET_Y
    )

    SetTimer(UpdateFollowToolTip, 16)
    TrayTip("ISBN pronto: " isbn, "Biblios")
}


AutoPasteHovered(isbn) {
    global suppressClipboard
    global overwriteExistingText
    global TOOLTIP_OFFSET_X
    global TOOLTIP_OFFSET_Y
    global tipHwnd
    global lastX
    global lastY
    global pendingISBN

    suppressClipboard := true

    try {
        SetTimer(UpdateFollowToolTip, 0)
        tipHwnd := 0
        lastX := -1
        lastY := -1
        pendingISBN := ""

        ; Click to focus the hovered input field
        Click()
        Sleep(40)

        A_Clipboard := isbn
        Sleep(30)

        ; Select all existing text if overwrite option is active
        if overwriteExistingText {
            SendInput("^a")
            Sleep(25)
        }
        SendInput("^v")

        PlayTapSound()

        MouseGetPos(&mx, &my)
        ToolTip("ISBN incollato: " isbn, mx + TOOLTIP_OFFSET_X, my + TOOLTIP_OFFSET_Y)

        SetTimer(ClearToolTip, -1200)

    } finally {
        SetTimer(ReleaseClipboardSuppression, -300)
    }
}


UpdateFollowToolTip() {
    global pendingISBN
    global tipHwnd
    global lastX
    global lastY
    global TOOLTIP_OFFSET_X
    global TOOLTIP_OFFSET_Y

    if (pendingISBN = "" || !tipHwnd) {
        SetTimer(UpdateFollowToolTip, 0)
        ToolTip()
        return
    }

    MouseGetPos(&mx, &my)

    if (mx = lastX && my = lastY)
        return

    lastX := mx
    lastY := my

    DllCall(
        "User32.dll\SetWindowPos",
        "Ptr", tipHwnd,
        "Ptr", 0,
        "Int", mx + TOOLTIP_OFFSET_X,
        "Int", my + TOOLTIP_OFFSET_Y,
        "Int", 0,
        "Int", 0,
        "UInt", 0x0015
    )
}


CancelISBN(*) {
    global pendingISBN
    global tipHwnd
    global lastX
    global lastY

    pendingISBN := ""
    tipHwnd := 0
    lastX := -1
    lastY := -1

    SetTimer(UpdateFollowToolTip, 0)
    ToolTip()
}


ClearToolTip() {
    global tipHwnd
    global lastX
    global lastY

    if (pendingISBN = "") {
        tipHwnd := 0
        lastX := -1
        lastY := -1
        SetTimer(UpdateFollowToolTip, 0)
        ToolTip()
    }
}


; ============================================================
; Mouse / Browser textbox paste
; ============================================================

HandleLeftClick(*) {
    global enabled
    global pendingISBN
    global TARGET_TAB_TITLE

    if !enabled
        return

    if (pendingISBN = "")
        return

    MouseGetPos(, , &windowID)

    if !windowID
        return

    title := ""
    try title := WinGetTitle("ahk_id " windowID)

    if (TARGET_TAB_TITLE != "") && !InStr(title, TARGET_TAB_TITLE, false)
        return

    wasIBeam := (A_Cursor = "IBeam")
    isbn := pendingISBN

    SetTimer(
        () => PastePendingISBN(isbn, wasIBeam),
        -100
    )
}


PastePendingISBN(isbn, wasIBeam) {
    global pendingISBN
    global suppressClipboard
    global tipHwnd
    global lastX
    global lastY
    global overwriteExistingText
    global TOOLTIP_OFFSET_X
    global TOOLTIP_OFFSET_Y

    if (pendingISBN != isbn)
        return

    if (!wasIBeam && A_Cursor != "IBeam")
        return

    suppressClipboard := true

    try {
        SetTimer(UpdateFollowToolTip, 0)
        tipHwnd := 0
        lastX := -1
        lastY := -1
        pendingISBN := ""

        A_Clipboard := isbn
        Sleep(40)

        if overwriteExistingText {
            SendInput("^a")
            Sleep(25)
        }
        SendInput("^v")

        PlayTapSound()

        MouseGetPos(&mx, &my)
        ToolTip("ISBN incollato: " isbn, mx + TOOLTIP_OFFSET_X, my + TOOLTIP_OFFSET_Y)

        SetTimer(ClearToolTip, -1200)

    } finally {
        SetTimer(ReleaseClipboardSuppression, -300)
    }
}


PlayTapSound() {
    global playTapSoundEnabled

    if !playTapSoundEnabled
        return

    tapFile := A_ScriptDir "\tap.wav"
    if FileExist(tapFile) {
        SoundPlay(tapFile)
    } else {
        navSound := A_WinDir "\Media\Windows Navigation Start.wav"
        if FileExist(navSound)
            SoundPlay(navSound)
        else
            SoundPlay("*-1")
    }
}


ReleaseClipboardSuppression() {
    global suppressClipboard
    suppressClipboard := false
}


; ============================================================
; Winsock Local Listener
; ============================================================

StartWinsock() {
    global winsockStarted
    wsadata := Buffer(512, 0)

    result := DllCall(
        "Ws2_32\WSAStartup",
        "UShort", 0x0202,
        "Ptr", wsadata.Ptr,
        "Int"
    )

    if (result != 0)
        return false

    winsockStarted := true
    return true
}


StartHttpServer() {
    global serverSocket
    global HTTP_PORT

    serverSocket := DllCall(
        "Ws2_32\socket",
        "Int", 2, ; AF_INET
        "Int", 1, ; SOCK_STREAM
        "Int", 6, ; IPPROTO_TCP
        "Ptr"
    )

    if (serverSocket = -1)
        return false

    mode := 1
    result := DllCall(
        "Ws2_32\ioctlsocket",
        "Ptr", serverSocket,
        "UInt", 0x8004667E, ; FIONBIO
        "UInt*", mode,
        "Int"
    )

    if (result != 0) {
        CloseSocket(serverSocket)
        serverSocket := 0
        return false
    }

    address := Buffer(16, 0)
    NumPut("UShort", 2, address, 0) ; AF_INET

    networkPort := DllCall("Ws2_32\htons", "UShort", HTTP_PORT, "UShort")
    NumPut("UShort", networkPort, address, 2)
    NumPut("UInt", 0, address, 4) ; 0.0.0.0

    result := DllCall(
        "Ws2_32\bind",
        "Ptr", serverSocket,
        "Ptr", address.Ptr,
        "Int", 16,
        "Int"
    )

    if (result != 0) {
        CloseSocket(serverSocket)
        serverSocket := 0
        return false
    }

    result := DllCall(
        "Ws2_32\listen",
        "Ptr", serverSocket,
        "Int", 8,
        "Int"
    )

    if (result != 0) {
        CloseSocket(serverSocket)
        serverSocket := 0
        return false
    }

    DebugLog("AutoHotkey local listener started on port " HTTP_PORT)
    return true
}


PollSockets() {
    global serverSocket
    global clientStates

    if !serverSocket
        return

    Loop {
        clientSocket := DllCall(
            "Ws2_32\accept",
            "Ptr", serverSocket,
            "Ptr", 0,
            "Ptr", 0,
            "Ptr"
        )

        if (clientSocket = -1)
            break

        mode := 1
        result := DllCall(
            "Ws2_32\ioctlsocket",
            "Ptr", clientSocket,
            "UInt", 0x8004667E,
            "UInt*", mode,
            "Int"
        )

        if (result != 0) {
            CloseSocket(clientSocket)
            continue
        }

        clientStates[clientSocket] := {
            data: "",
            headersComplete: false,
            contentLength: 0,
            continueSent: false
        }
    }

    sockets := []
    for socket, state in clientStates
        sockets.Push(socket)

    for _, socket in sockets {
        if clientStates.Has(socket)
            PollClient(socket)
    }
}


PollClient(socket) {
    global clientStates

    if !clientStates.Has(socket)
        return

    state := clientStates[socket]
    recvBuffer := Buffer(4096, 0)

    Loop 4 {
        bytesReceived := DllCall(
            "Ws2_32\recv",
            "Ptr", socket,
            "Ptr", recvBuffer.Ptr,
            "Int", recvBuffer.Size,
            "Int", 0,
            "Int"
        )

        if (bytesReceived = -1) {
            error := DllCall("Ws2_32\WSAGetLastError", "Int")
            if (error = 10035) ; WSAEWOULDBLOCK
                return
            CloseClient(socket)
            return
        }

        if (bytesReceived = 0) {
            CloseClient(socket)
            return
        }

        chunk := StrGet(recvBuffer.Ptr, bytesReceived, "UTF-8")
        state.data .= chunk

        if (StrLen(state.data) > 16384) {
            SendHttpResponse(socket, 413, "Payload Too Large", "Request too large")
            CloseClient(socket)
            return
        }

        headerEnd := InStr(state.data, "`r`n`r`n")
        if !headerEnd
            continue

        if !state.headersComplete {
            headers := SubStr(state.data, 1, headerEnd + 3)
            state.headersComplete := true

            if RegExMatch(headers, "im)^Content-Length:\s*(\d+)", &lengthMatch)
                state.contentLength := Integer(lengthMatch[1])
            else
                state.contentLength := 0

            if RegExMatch(headers, "im)^Expect:\s*100-continue\s*$") {
                if !state.continueSent {
                    SendRaw(socket, "HTTP/1.1 100 Continue`r`n`r`n")
                    state.continueSent := true
                }
            }
        }

        body := SubStr(state.data, headerEnd + 4)
        bodyLength := StrLen(body)

        if (bodyLength >= state.contentLength) {
            request := state.data
            HandleHttpRequest(socket, request, headerEnd, state.contentLength)
            return
        }
    }
}


HandleHttpRequest(socket, request, headerEnd, contentLength) {
    lineEnd := InStr(request, "`r`n")
    if !lineEnd {
        SendHttpResponse(socket, 400, "Bad Request", "Invalid HTTP request")
        CloseClient(socket)
        return
    }

    requestLine := SubStr(request, 1, lineEnd - 1)

    ; Action: Show Centered QR
    if RegExMatch(requestLine, "^POST\s+/qr/show(?:\?| )") {
        ShowCenteredQR()
        SendHttpResponse(socket, 200, "OK", "QR shown")
        CloseClient(socket)
        return
    }

    ; Action: Hide Centered QR
    if RegExMatch(requestLine, "^POST\s+/qr/hide(?:\?| )") {
        HideCenteredQR()
        SendHttpResponse(socket, 200, "OK", "QR hidden")
        CloseClient(socket)
        return
    }

    ; Action: Paste / ISBN
    if !RegExMatch(requestLine, "^POST\s+/(?:isbn|paste)(?:\?| )", &routeMatch) {
        SendHttpResponse(socket, 405, "Method Not Allowed", "Method Not Allowed")
        CloseClient(socket)
        return
    }

    isPasteRoute := RegExMatch(requestLine, "^POST\s+/paste(?:\?| )")

    ; Token verification (only required for direct legacy /isbn connections)
    if !isPasteRoute {
        if !RegExMatch(request, "im)^Biblios-Token:\s*(.+?)\s*$", &tokenMatch) {
            SendHttpResponse(socket, 401, "Unauthorized", "Unauthorized")
            CloseClient(socket)
            return
        }

        receivedToken := Trim(tokenMatch[1])
        validToken := BIBLIOS_TOKEN
        if FileExist(TOKEN_FILE) {
            try {
                t := Trim(FileRead(TOKEN_FILE, "UTF-8"))
                if (t != "")
                    validToken := t
            }
        }

        if !SecureTokenCompare(receivedToken, validToken) {
            SendHttpResponse(socket, 401, "Unauthorized", "Unauthorized")
            CloseClient(socket)
            return
        }
    }

    body := SubStr(request, headerEnd + 4)
    if (contentLength >= 0)
        body := SubStr(body, 1, contentLength)
    body := Trim(body)

    if !body {
        SendHttpResponse(socket, 400, "Bad Request", "Missing ISBN")
        CloseClient(socket)
        return
    }

    isbn := NormalizeISBN(body)
    if !IsISBNFormatValid(isbn) {
        SendHttpResponse(socket, 400, "Bad Request", "Invalid ISBN")
        CloseClient(socket)
        return
    }

    ArmISBN(isbn)
    SendHttpResponse(socket, 200, "OK", "OK")
    CloseClient(socket)
}


SendHttpResponse(socket, statusCode, reason, body) {
    response := "HTTP/1.1 " . statusCode . " " . reason . "`r`n"
        . "Content-Type: text/plain; charset=utf-8`r`n"
        . "Content-Length: " . StrLen(body) . "`r`n"
        . "Connection: close`r`n`r`n"
        . body

    SendRaw(socket, response)
}


SendRaw(socket, text) {
    bytes := StrPut(text, "UTF-8")
    sendBuffer := Buffer(bytes, 0)
    StrPut(text, sendBuffer, "UTF-8")

    byteCount := bytes - 1
    DllCall(
        "Ws2_32\send",
        "Ptr", socket,
        "Ptr", sendBuffer.Ptr,
        "Int", byteCount,
        "Int", 0,
        "Int"
    )
}


SecureTokenCompare(a, b) {
    aLength := StrLen(a)
    bLength := StrLen(b)
    if (aLength != bLength)
        return false

    difference := 0
    Loop aLength {
        difference |= Ord(SubStr(a, A_Index, 1)) ^ Ord(SubStr(b, A_Index, 1))
    }
    return difference = 0
}


CloseClient(socket) {
    global clientStates
    if clientStates.Has(socket)
        clientStates.Delete(socket)
    CloseSocket(socket)
}


CloseSocket(socket) {
    if socket
        DllCall("Ws2_32\closesocket", "Ptr", socket, "Int")
}


Shutdown(*) {
    global serverSocket
    global clientStates
    global winsockStarted
    global qrGui

    SetTimer(PollSockets, 0)
    SetTimer(UpdateFollowToolTip, 0)

    if qrGui {
        qrGui.Destroy()
        qrGui := 0
    }

    for socket, state in clientStates
        CloseSocket(socket)
    clientStates.Clear()

    if serverSocket {
        CloseSocket(serverSocket)
        serverSocket := 0
    }

    if winsockStarted {
        DllCall("Ws2_32\WSACleanup")
        winsockStarted := false
    }
}


DebugLog(message) {
    global DEBUG_LOG
    try {
        timestamp := FormatTime(, "HH:mm:ss")
        message := RegExReplace(message, "(?im)(Biblios-Token:\s*)[^\r\n]+", "$1<TOKEN>")
        FileAppend(timestamp " | " message "`n", DEBUG_LOG, "UTF-8")
    }
}