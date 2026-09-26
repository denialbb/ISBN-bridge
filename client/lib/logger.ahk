#Requires AutoHotkey v2.0

class Logger {
    static logPath := ""

    static Init(path := "") {
        if (path != "") {
            this.logPath := path
        } else if FileExist(A_ScriptDir "\biblios-debug.log") {
            this.logPath := A_ScriptDir "\biblios-debug.log"
        } else {
            this.logPath := A_ScriptDir "\..\biblios-debug.log"
        }
    }

    static Log(message) {
        if (this.logPath = "")
            this.Init()

        try {
            timestamp := FormatTime(, "HH:mm:ss")
            FileAppend(timestamp " | " message "`n", this.logPath, "UTF-8")
        }
    }

    static Open() {
        if (this.logPath = "")
            this.Init()
        if !FileExist(this.logPath)
            FileAppend("", this.logPath, "UTF-8")
        Run(this.logPath)
    }
}
