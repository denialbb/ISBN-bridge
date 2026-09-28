#Requires AutoHotkey v2.0

class ISBNValidator {
    static Normalize(raw) {
        clean := ""
        Loop Parse, raw {
            c := A_LoopField
            if RegExMatch(c, "[0-9]")
                clean .= c
            else if (c = "x" || c = "X")
                clean .= "X"
            else if (c = " " || c = "`t" || c = "`r" || c = "`n" || c = "-")
                continue
            else
                clean .= c
        }
        return clean
    }

    static Validate(raw) {
        clean := this.Normalize(raw)
        len := StrLen(clean)
        if (len = 13) {
            if this.ValidateISBN13(clean)
                return clean
        } else if (len = 10) {
            if this.ValidateISBN10(clean)
                return clean
        }
        return ""
    }

    static ValidateISBN13(s) {
        if !RegExMatch(s, "^[0-9]{13}$")
            return false
        sum := 0
        Loop 13 {
            digit := Integer(SubStr(s, A_Index, 1))
            sum += (Mod(A_Index - 1, 2) = 0) ? digit : (digit * 3)
        }
        return Mod(sum, 10) = 0
    }

    static ValidateISBN10(s) {
        if !RegExMatch(s, "^[0-9]{9}[0-9X]$")
            return false
        sum := 0
        Loop 9 {
            digit := Integer(SubStr(s, A_Index, 1))
            sum += digit * (11 - A_Index)
        }
        last := SubStr(s, 10, 1)
        checkVal := (last = "X") ? 10 : Integer(last)
        sum += checkVal
        return Mod(sum, 11) = 0
    }
}
