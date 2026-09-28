#Requires AutoHotkey v2.0

class HttpListener {
    static serverSocket := 0
    static clientStates := Map()
    static winsockStarted := false
    static pollTimer := ObjBindMethod(HttpListener, "PollSockets")

    static Start(port) {
        if !this.StartWinsock()
            return false

        ; AF_INET = 2, SOCK_STREAM = 1, IPPROTO_TCP = 6
        this.serverSocket := DllCall("Ws2_32\socket", "Int", 2, "Int", 1, "Int", 6, "Ptr")
        if (this.serverSocket = -1)
            return false

        ; Set non-blocking mode (FIONBIO = 0x8004667E)
        mode := 1
        if (DllCall("Ws2_32\ioctlsocket", "Ptr", this.serverSocket, "UInt", 0x8004667E, "UInt*", mode, "Int") != 0) {
            this.CloseSocket(this.serverSocket)
            return false
        }

        ; Enable SO_REUSEADDR (SOL_SOCKET = 0xFFFF, SO_REUSEADDR = 0x0004)
        reuseOpt := Buffer(4, 0)
        NumPut("Int", 1, reuseOpt, 0)
        DllCall("Ws2_32\setsockopt", "Ptr", this.serverSocket, "Int", 0xFFFF, "Int", 0x0004, "Ptr", reuseOpt.Ptr, "Int", 4, "Int")

        ; Bind to 127.0.0.1 (localhost only). The paste endpoint takes
        ; unauthenticated body text, so it must never reach the LAN.
        ; Bytes are stored in network order: 127.0.0.1 (a plain
        ; NumPut("UInt", 0x7F000001) would land little-endian as
        ; 1.0.0.127 and fail with WSAEADDRNOTAVAIL).
        address := Buffer(16, 0)
        NumPut("UShort", 2, address, 0) ; AF_INET
        netPort := DllCall("Ws2_32\htons", "UShort", port, "UShort")
        NumPut("UShort", netPort, address, 2)
        NumPut("UChar", 127, address, 4)
        NumPut("UChar", 0, address, 5)
        NumPut("UChar", 0, address, 6)
        NumPut("UChar", 1, address, 7)

        bound := false
        Loop 10 {
            if (DllCall("Ws2_32\bind", "Ptr", this.serverSocket, "Ptr", address.Ptr, "Int", 16, "Int") = 0) {
                bound := true
                break
            }
            Sleep(200)
        }

        if !bound {
            this.CloseSocket(this.serverSocket)
            return false
        }

        if (DllCall("Ws2_32\listen", "Ptr", this.serverSocket, "Int", 8, "Int") != 0) {
            this.CloseSocket(this.serverSocket)
            return false
        }

        SetTimer(this.pollTimer, 25)
        Logger.Log("Desktop local listener active on port " port)
        return true
    }

    static StartWinsock() {
        wsadata := Buffer(512, 0)
        if (DllCall("Ws2_32\WSAStartup", "UShort", 0x0202, "Ptr", wsadata.Ptr, "Int") != 0)
            return false
        this.winsockStarted := true
        return true
    }

    static PollSockets() {
        if !this.serverSocket
            return

        ; Accept pending clients
        Loop {
            clientSocket := DllCall("Ws2_32\accept", "Ptr", this.serverSocket, "Ptr", 0, "Ptr", 0, "Ptr")
            if (clientSocket = -1)
                break

            mode := 1
            DllCall("Ws2_32\ioctlsocket", "Ptr", clientSocket, "UInt", 0x8004667E, "UInt*", mode, "Int")

            this.clientStates[clientSocket] := {
                data: "",
                headersComplete: false,
                contentLength: 0,
                connectTick: A_TickCount
            }
        }

        now := A_TickCount
        sockets := []
        for sock, state in this.clientStates
            sockets.Push(sock)

        for _, sock in sockets {
            if !this.clientStates.Has(sock)
                continue
            ; 5-second deadline to prevent connection descriptor leaks
            if (now - this.clientStates[sock].connectTick > 5000) {
                this.CloseClient(sock)
                continue
            }
            this.PollClient(sock)
        }
    }

    static PollClient(sock) {
        if !this.clientStates.Has(sock)
            return

        state := this.clientStates[sock]
        recvBuf := Buffer(4096, 0)

        bytesReceived := DllCall("Ws2_32\recv", "Ptr", sock, "Ptr", recvBuf.Ptr, "Int", recvBuf.Size, "Int", 0, "Int")

        if (bytesReceived = -1) {
            ; WSAEWOULDBLOCK = 10035
            if (DllCall("Ws2_32\WSAGetLastError", "Int") = 10035)
                return
            this.CloseClient(sock)
            return
        }

        if (bytesReceived <= 0) {
            this.CloseClient(sock)
            return
        }

        state.data .= StrGet(recvBuf.Ptr, bytesReceived, "UTF-8")

        if (StrLen(state.data) > 16384) {
            this.SendResponse(sock, 413, "Payload Too Large", "Request too large")
            this.CloseClient(sock)
            return
        }

        headerEnd := InStr(state.data, "`r`n`r`n")
        if !headerEnd
            return

        if !state.headersComplete {
            headers := SubStr(state.data, 1, headerEnd + 3)
            state.headersComplete := true

            if RegExMatch(headers, "im)^Content-Length:\s*(\d+)", &lengthMatch)
                state.contentLength := Integer(lengthMatch[1])
        }

        body := SubStr(state.data, headerEnd + 4)
        if (StrLen(body) >= state.contentLength) {
            this.HandleRequest(sock, state.data, headerEnd, state.contentLength)
        }
    }

    static HandleRequest(sock, request, headerEnd, contentLength) {
        lineEnd := InStr(request, "`r`n")
        if !lineEnd {
            this.SendResponse(sock, 400, "Bad Request", "Malformed request")
            this.CloseClient(sock)
            return
        }

        ; Every request must carry the Go forwarder's shared secret
        ; (local_secret.txt, written by the server at startup). The
        ; listener only binds localhost, so this is defense in depth
        ; against other local processes — not the primary barrier.
        if !this.HasLocalSecret(request, headerEnd) {
            Logger.Log("Rejected request without forwarder secret")
            this.SendResponse(sock, 403, "Forbidden", "Missing forwarder secret")
            this.CloseClient(sock)
            return
        }

        requestLine := SubStr(request, 1, lineEnd - 1)

        ; Action: Show Centered QR
        if RegExMatch(requestLine, "^POST\s+/qr/show(?:\?| )") {
            try {
                QRModal.Show()
                this.SendResponse(sock, 200, "OK", "QR shown")
            } catch as err {
                Logger.Log("Error showing QR modal: " err.Message)
                this.SendResponse(sock, 500, "Internal Server Error", err.Message)
            }
            this.CloseClient(sock)
            return
        }

        ; Action: Hide Centered QR
        if RegExMatch(requestLine, "^POST\s+/qr/hide(?:\?| )") {
            try {
                QRModal.Hide()
                this.SendResponse(sock, 200, "OK", "QR hidden")
            } catch as err {
                Logger.Log("Error hiding QR modal: " err.Message)
                this.SendResponse(sock, 500, "Internal Server Error", err.Message)
            }
            this.CloseClient(sock)
            return
        }

        ; Action: Paste / ISBN
        if !RegExMatch(requestLine, "^POST\s+/(?:paste|isbn)(?:\?| )") {
            this.SendResponse(sock, 405, "Method Not Allowed", "Method Not Allowed")
            this.CloseClient(sock)
            return
        }

        body := SubStr(request, headerEnd + 4)
        if (contentLength >= 0)
            body := SubStr(body, 1, contentLength)
        rawISBN := Trim(body)

        if (rawISBN = "") {
            this.SendResponse(sock, 400, "Bad Request", "Missing ISBN")
            this.CloseClient(sock)
            return
        }

        ; Strictly validate ISBN checksum and format before arming
        validatedISBN := ISBNValidator.Validate(rawISBN)
        if (validatedISBN = "") {
            Logger.Log("Rejected invalid ISBN from local request")
            this.SendResponse(sock, 400, "Bad Request", "Invalid ISBN checksum or format")
            this.CloseClient(sock)
            return
        }

        Logger.Log("Received verified ISBN: " validatedISBN)

        try {
            PasteEngine.Arm(validatedISBN)
            this.SendResponse(sock, 200, "OK", "OK")
        } catch as err {
            Logger.Log("Error arming paste engine: " err.Message)
            this.SendResponse(sock, 500, "Internal Server Error", err.Message)
        }

        this.CloseClient(sock)
    }

    static FindSecretFile() {
        if FileExist(A_ScriptDir "\local_secret.txt")
            return A_ScriptDir "\local_secret.txt"
        if FileExist(A_ScriptDir "\..\local_secret.txt")
            return A_ScriptDir "\..\local_secret.txt"
        return ""
    }

    ; Fail closed: verify secret strictly in constant time
    static HasLocalSecret(request, headerEnd) {
        path := this.FindSecretFile()
        if (path = "") {
            Logger.Log("Rejected request: local_secret.txt not found")
            return false
        }
        try
            expected := Trim(FileRead(path, "UTF-8"))
        catch {
            return false
        }
        if (expected = "")
            return false
        headers := SubStr(request, 1, headerEnd + 3)
        if RegExMatch(headers, "im)^X-ISBN-Bridge-Local:\s*(\S+)", &m)
            return this.ConstantTimeCompare(m[1], expected)
        return false
    }

    static ConstantTimeCompare(a, b) {
        if StrLen(a) != StrLen(b)
            return false
        diff := 0
        Loop StrLen(a) {
            diff |= Ord(SubStr(a, A_Index, 1)) ^ Ord(SubStr(b, A_Index, 1))
        }
        return diff = 0
    }

    static SendResponse(sock, statusCode, reason, body) {
        resp := "HTTP/1.1 " statusCode " " reason "`r`n"
            . "Content-Type: text/plain; charset=utf-8`r`n"
            . "Content-Length: " StrLen(body) "`r`n"
            . "Connection: close`r`n`r`n"
            . body

        bytes := StrPut(resp, "UTF-8")
        buf := Buffer(bytes, 0)
        StrPut(resp, buf, "UTF-8")
        DllCall("Ws2_32\send", "Ptr", sock, "Ptr", buf.Ptr, "Int", bytes - 1, "Int", 0, "Int")
    }

    static CloseClient(sock) {
        if this.clientStates.Has(sock)
            this.clientStates.Delete(sock)
        this.CloseSocket(sock)
    }

    static CloseSocket(sock) {
        if sock
            DllCall("Ws2_32\closesocket", "Ptr", sock, "Int")
    }

    static Shutdown() {
        SetTimer(this.pollTimer, 0)

        for sock, state in this.clientStates
            this.CloseSocket(sock)
        this.clientStates.Clear()

        if this.serverSocket {
            this.CloseSocket(this.serverSocket)
            this.serverSocket := 0
        }

        if this.winsockStarted {
            DllCall("Ws2_32\WSACleanup")
            this.winsockStarted := false
        }
    }
}
