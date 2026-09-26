#Requires AutoHotkey v2.0
#SingleInstance Force

; ============================================================
; Biblios Token Generator
; Generates a cryptographically secure random token,
; saves it to token.txt and copies it to clipboard.
; ============================================================

TOKEN_FILE := A_ScriptDir "\token.txt"
TOKEN_LENGTH := 48

GenerateAndShowToken()

GenerateAndShowToken() {
    token := CreateSecureToken(TOKEN_LENGTH)

    ; Save to token.txt
    try {
        if FileExist(TOKEN_FILE)
            FileDelete(TOKEN_FILE)
        FileAppend(token, TOKEN_FILE, "UTF-8")
    } catch as err {
        MsgBox("Errore nel salvataggio di token.txt: " err.Message, "Errore", "Iconx")
    }

    ; Copy to clipboard
    A_Clipboard := token

    ; Show GUI
    mainGui := Gui("+AlwaysOnTop", "Biblios - Token Generator")
    mainGui.SetFont("s10", "Segoe UI")

    mainGui.Add("Text", "w420", "Un nuovo token di sicurezza è stato generato e salvato in token.txt:`n(È già stato copiato negli appunti)")

    editBox := mainGui.Add("Edit", "w420 r2 ReadOnly vTokenEdit", token)
    editBox.SetFont("s10", "Consolas")

    btnCopy := mainGui.Add("Button", "w130", "Copia negli appunti")
    btnCopy.OnEvent("Click", (*) => (A_Clipboard := editBox.Value, TrayTip("Token copiato negli appunti!", "Biblios")))

    btnRegen := mainGui.Add("Button", "x+10 w130", "Rigenera")
    btnRegen.OnEvent("Click", (*) => Regenerate(editBox))

    btnClose := mainGui.Add("Button", "x+10 w130", "Chiudi")
    btnClose.OnEvent("Click", (*) => mainGui.Destroy())

    mainGui.OnEvent("Close", (*) => mainGui.Destroy())
    mainGui.Show()
}

Regenerate(editCtrl) {
    newToken := CreateSecureToken(TOKEN_LENGTH)
    try {
        if FileExist(TOKEN_FILE)
            FileDelete(TOKEN_FILE)
        FileAppend(newToken, TOKEN_FILE, "UTF-8")
    }
    editCtrl.Value := newToken
    A_Clipboard := newToken
    TrayTip("Nuovo token generato e copiato!", "Biblios")
}

CreateSecureToken(length := 48) {
    chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
    charLen := StrLen(chars)

    ; Allocate buffer for cryptographically secure random bytes
    buf := Buffer(length, 0)

    ; RtlGenRandom / SystemFunction036 from Advapi32
    result := DllCall("Advapi32.dll\SystemFunction036", "Ptr", buf.Ptr, "UInt", length, "Int")
    if !result {
        ; Fallback to pseudorandom if RtlGenRandom fails
        token := ""
        Loop length
            token .= SubStr(chars, Random(1, charLen), 1)
        return token
    }

    token := ""
    Loop length {
        byteVal := NumGet(buf, A_Index - 1, "UChar")
        token .= SubStr(chars, Mod(byteVal, charLen) + 1, 1)
    }

    return token
}
