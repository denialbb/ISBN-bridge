#Requires AutoHotkey v2.0
#SingleInstance Force
Persistent

CoordMode("ToolTip", "Screen")
CoordMode("Mouse", "Screen")

; ============================================================
; Biblios ISBN Auto-Paste
; ============================================================

; ----------------------------
; Configuration
; ----------------------------

global TARGET_TAB_TITLE := "hardcover"
global HTTP_PORT := 8766

; Replace this with your actual token or use 'Generate Token.ahk'
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

global serverSocket := 0
global clientStates := Map()
global winsockStarted := false


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
        "Impossibile avviare il server HTTP sulla porta "
        HTTP_PORT ".`n`n"
        "La porta potrebbe essere già in uso.",
        "Biblios",
        "Iconx"
    )
    ExitApp
}

OnClipboardChange(ClipChanged)

; Poll the non-blocking sockets.
SetTimer(PollSockets, 25)

; Check mouse clicks.
Hotkey("~LButton", HandleLeftClick)

; ESC cancels the pending ISBN.
Hotkey("~Esc", CancelISBN)

CreateTrayMenu()

TrayTip(
    "Server HTTP attivo sulla porta " HTTP_PORT,
    "Biblios"
)


; ============================================================
; Tray menu
; ============================================================

CreateTrayMenu() {
    A_TrayMenu.Delete()

    A_TrayMenu.Add(
        "Attivo",
        ToggleEnabled
    )

    A_TrayMenu.Add()

    A_TrayMenu.Add(
        "Svuota ISBN",
        CancelISBN
    )

    A_TrayMenu.Add(
        "Genera nuovo token",
        (*) => Run('"' A_AhkPath '" "' A_ScriptDir '\Generate Token.ahk"')
    )

    A_TrayMenu.Add(
        "Apri log debug",
        OpenDebugLog
    )

    A_TrayMenu.Add()

    A_TrayMenu.Add(
        "Esci",
        (*) => ExitApp()
    )

    UpdateTrayState()
}


ToggleEnabled(*) {
    global enabled

    enabled := !enabled
    UpdateTrayState()

    if enabled {
        TrayTip(
            "Auto-incolla attivo.",
            "Biblios"
        )
    } else {
        CancelISBN()
        TrayTip(
            "Auto-incolla disattivato.",
            "Biblios"
        )
    }
}


UpdateTrayState() {
    global enabled

    A_TrayMenu.Uncheck("Attivo")

    if enabled
        A_TrayMenu.Check("Attivo")
}


OpenDebugLog(*) {
    global DEBUG_LOG

    if !FileExist(DEBUG_LOG)
        FileAppend("", DEBUG_LOG, "UTF-8")

    Run(DEBUG_LOG)
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

    ; Remove spaces and hyphens.
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

    pendingISBN := isbn

    ; Cancel any pending auto-hide timer from a previous paste.
    SetTimer(ClearToolTip, 0)

    MouseGetPos(&mx, &my)
    lastX := mx
    lastY := my

    ; Create the tooltip window once and store its HWND.
    tipHwnd := ToolTip(
        "ISBN pronto: " isbn "`n"
        "Clicca nel campo di testo per incollarlo.`n"
        "ESC per annullare.",
        mx + 20,
        my + 20
    )

    SetTimer(UpdateFollowToolTip, 16)

    TrayTip(
        "ISBN pronto: " isbn,
        "Biblios"
    )
}


UpdateFollowToolTip() {
    global pendingISBN
    global tipHwnd
    global lastX
    global lastY

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

    ; Move the existing tooltip window natively without redrawing text.
    ; 0x0015 = SWP_NOSIZE (0x0001) | SWP_NOZORDER (0x0004) | SWP_NOACTIVATE (0x0010)
    DllCall(
        "User32.dll\SetWindowPos",
        "Ptr", tipHwnd,
        "Ptr", 0,
        "Int", mx + 20,
        "Int", my + 20,
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

    ; Case-insensitive check for TARGET_TAB_TITLE in the window/tab title.
    if (TARGET_TAB_TITLE != "") && !InStr(title, TARGET_TAB_TITLE, false)
        return

    wasIBeam := (A_Cursor = "IBeam")
    isbn := pendingISBN

    ; Let the browser process the mouse click and focus the input field first.
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

    ; User may have cancelled or armed a new ISBN during the 100 ms delay.
    if (pendingISBN != isbn)
        return

    ; Ensure the click landed inside a text input field (cursor was or became IBeam).
    if (!wasIBeam && A_Cursor != "IBeam")
        return

    suppressClipboard := true

    try {
        ; Stop the follow timer before changing the tooltip text.
        SetTimer(UpdateFollowToolTip, 0)
        tipHwnd := 0
        lastX := -1
        lastY := -1
        pendingISBN := ""

        A_Clipboard := isbn
        Sleep(40)

        ; Select all existing text in the input box and paste over it.
        SendInput("^a")
        Sleep(30)
        SendInput("^v")

        MouseGetPos(&mx, &my)
        ToolTip("ISBN incollato: " isbn, mx + 20, my + 20)

        SetTimer(
            ClearToolTip,
            -1200
        )

    } finally {
        SetTimer(
            ReleaseClipboardSuppression,
            -300
        )
    }
}


ReleaseClipboardSuppression() {
    global suppressClipboard
    suppressClipboard := false
}


; ============================================================
; Winsock
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

    ; AF_INET = 2
    ; SOCK_STREAM = 1
    ; IPPROTO_TCP = 6

    serverSocket := DllCall(
        "Ws2_32\socket",
        "Int", 2,
        "Int", 1,
        "Int", 6,
        "Ptr"
    )

    if (serverSocket = -1)
        return false

    ; Make listener non-blocking.
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

    ; sockaddr_in
    address := Buffer(16, 0)

    ; AF_INET
    NumPut("UShort", 2, address, 0)

    ; Port in network byte order.
    networkPort := DllCall(
        "Ws2_32\htons",
        "UShort", HTTP_PORT,
        "UShort"
    )

    NumPut("UShort", networkPort, address, 2)

    ; 0.0.0.0 = listen on all IPv4 interfaces.
    NumPut("UInt", 0, address, 4)

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

    DebugLog(
        "HTTP server started on 0.0.0.0:" HTTP_PORT
    )

    return true
}


; ============================================================
; Socket polling
; ============================================================

PollSockets() {
    global serverSocket
    global clientStates

    if !serverSocket
        return

    ; Accept all currently waiting clients.
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

        ; Make client socket non-blocking.
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

        DebugLog(
            "Accepted client socket: " clientSocket
        )
    }

    ; Process existing clients.
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

        ; No data currently available.
        if (bytesReceived = -1) {
            error := DllCall(
                "Ws2_32\WSAGetLastError",
                "Int"
            )

            ; WSAEWOULDBLOCK = 10035
            if (error = 10035)
                return

            DebugLog(
                "recv error: " error
            )

            CloseClient(socket)
            return
        }

        ; Client closed connection.
        if (bytesReceived = 0) {
            DebugLog(
                "Client closed socket: " socket
            )

            CloseClient(socket)
            return
        }

        chunk := StrGet(
            recvBuffer.Ptr,
            bytesReceived,
            "UTF-8"
        )

        state.data .= chunk

        DebugLog(
            "Socket " socket
            " received " bytesReceived
            " bytes"
        )

        if (StrLen(state.data) > 16384) {
            DebugLog("Request too large")
            SendHttpResponse(
                socket,
                413,
                "Payload Too Large",
                "Request too large"
            )
            CloseClient(socket)
            return
        }

        ; Wait until the complete HTTP header exists.
        headerEnd := InStr(
            state.data,
            "`r`n`r`n"
        )

        if !headerEnd
            continue

        if !state.headersComplete {
            headers := SubStr(
                state.data,
                1,
                headerEnd + 3
            )

            state.headersComplete := true

            if RegExMatch(
                headers,
                "im)^Content-Length:\s*(\d+)",
                &lengthMatch
            ) {
                state.contentLength := Integer(
                    lengthMatch[1]
                )
            } else {
                state.contentLength := 0
            }

            DebugLog(
                "Content-Length: "
                state.contentLength
            )

            ; Handle HTTP 100-continue.
            if RegExMatch(
                headers,
                "im)^Expect:\s*100-continue\s*$"
            ) {
                if !state.continueSent {
                    SendRaw(
                        socket,
                        "HTTP/1.1 100 Continue`r`n`r`n"
                    )

                    state.continueSent := true

                    DebugLog(
                        "Sent 100 Continue"
                    )
                }
            }
        }

        ; Body begins after CRLFCRLF.
        body := SubStr(
            state.data,
            headerEnd + 4
        )

        bodyLength := StrLen(body)

        DebugLog(
            "Body received: "
            bodyLength
            "/"
            state.contentLength
        )

        ; Complete request.
        if (bodyLength >= state.contentLength) {
            request := state.data

            DebugLog(
                "Complete HTTP request received"
            )

            HandleHttpRequest(
                socket,
                request,
                headerEnd,
                state.contentLength
            )

            return
        }
    }
}


; ============================================================
; HTTP request handling
; ============================================================

HandleHttpRequest(
    socket,
    request,
    headerEnd,
    contentLength
) {
    DebugLog("=== HANDLE REQUEST ===")
    DebugLog("Request: [" request "]")

    ; ----------------------------
    ; Request line
    ; ----------------------------

    lineEnd := InStr(
        request,
        "`r`n"
    )

    if !lineEnd {
        SendHttpResponse(
            socket,
            400,
            "Bad Request",
            "Invalid HTTP request"
        )

        CloseClient(socket)
        return
    }

    requestLine := SubStr(
        request,
        1,
        lineEnd - 1
    )

    DebugLog(
        "Request line: [" requestLine "]"
    )

    ; ----------------------------
    ; Method/path
    ; ----------------------------

    if !RegExMatch(
        requestLine,
        "^POST\s+/(?:isbn|paste)(?:\?| )",
        &routeMatch
    ) {
        DebugLog("Route rejected")

        SendHttpResponse(
            socket,
            405,
            "Method Not Allowed",
            "Method Not Allowed"
        )

        CloseClient(socket)
        return
    }

    isPasteRoute := RegExMatch(requestLine, "^POST\s+/paste(?:\?| )")

    ; ----------------------------
    ; Token (only required if not /paste)
    ; ----------------------------

    if !isPasteRoute {
        if !RegExMatch(
            request,
            "im)^Biblios-Token:\s*(.+?)\s*$",
            &tokenMatch
        ) {
            DebugLog("Token missing")

            SendHttpResponse(
                socket,
                401,
                "Unauthorized",
                "Unauthorized"
            )

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

        if !SecureTokenCompare(
            receivedToken,
            validToken
        ) {
            DebugLog("Token rejected")

            SendHttpResponse(
                socket,
                401,
                "Unauthorized",
                "Unauthorized"
            )

            CloseClient(socket)
            return
        }

        DebugLog("Token accepted")
    }

    ; ----------------------------
    ; Body
    ; ----------------------------

    body := SubStr(
        request,
        headerEnd + 4
    )

    ; Only use Content-Length bytes/chars.
    if (contentLength >= 0)
        body := SubStr(
            body,
            1,
            contentLength
        )

    body := Trim(body)

    DebugLog(
        "ISBN body: [" body "]"
    )

    if !body {
        SendHttpResponse(
            socket,
            400,
            "Bad Request",
            "Missing ISBN"
        )

        CloseClient(socket)
        return
    }

    isbn := NormalizeISBN(body)

    if !IsISBNFormatValid(isbn) {
        DebugLog(
            "Invalid ISBN: [" isbn "]"
        )

        SendHttpResponse(
            socket,
            400,
            "Bad Request",
            "Invalid ISBN"
        )

        CloseClient(socket)
        return
    }

    ; ----------------------------
    ; Success
    ; ----------------------------

    DebugLog(
        "Valid ISBN received: " isbn
    )

    ArmISBN(isbn)

    SendHttpResponse(
        socket,
        200,
        "OK",
        "OK"
    )

    CloseClient(socket)
}


; ============================================================
; HTTP responses
; ============================================================

SendHttpResponse(
    socket,
    statusCode,
    reason,
    body
) {
    response := "HTTP/1.1 "
        . statusCode
        . " "
        . reason
        . "`r`n"
        . "Content-Type: text/plain; charset=utf-8`r`n"
        . "Content-Length: "
        . StrLen(body)
        . "`r`n"
        . "Connection: close`r`n"
        . "`r`n"
        . body

    SendRaw(socket, response)
}


SendRaw(socket, text) {
    bytes := StrPut(
        text,
        "UTF-8"
    )

    sendBuffer := Buffer(bytes, 0)

    StrPut(
        text,
        sendBuffer,
        "UTF-8"
    )

    ; Do not send the terminating null byte.
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


; ============================================================
; Token comparison
; ============================================================

SecureTokenCompare(a, b) {
    aLength := StrLen(a)
    bLength := StrLen(b)

    if (aLength != bLength)
        return false

    difference := 0

    Loop aLength {
        difference |= Ord(
            SubStr(a, A_Index, 1)
        ) ^ Ord(
            SubStr(b, A_Index, 1)
        )
    }

    return difference = 0
}


; ============================================================
; Socket cleanup
; ============================================================

CloseClient(socket) {
    global clientStates

    if clientStates.Has(socket)
        clientStates.Delete(socket)

    CloseSocket(socket)
}


CloseSocket(socket) {
    if socket
        DllCall(
            "Ws2_32\closesocket",
            "Ptr", socket,
            "Int"
        )
}


Shutdown(*) {
    global serverSocket
    global clientStates
    global winsockStarted

    SetTimer(PollSockets, 0)
    SetTimer(UpdateFollowToolTip, 0)

    for socket, state in clientStates
        CloseSocket(socket)

    clientStates.Clear()

    if serverSocket {
        CloseSocket(serverSocket)
        serverSocket := 0
    }

    if winsockStarted {
        DllCall(
            "Ws2_32\WSACleanup"
        )

        winsockStarted := false
    }
}


; ============================================================
; Debug logging
; ============================================================

DebugLog(message) {
    global DEBUG_LOG

    try {
        timestamp := FormatTime(
            ,
            "HH:mm:ss"
        )

        ; Avoid logging the token itself.
        message := RegExReplace(
            message,
            "(?im)(Biblios-Token:\s*)[^\r\n]+",
            "$1<TOKEN>"
        )

        FileAppend(
            timestamp
            " | "
            message
            "`n",
            DEBUG_LOG,
            "UTF-8"
        )
    }
}