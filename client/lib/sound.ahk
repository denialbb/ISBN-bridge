#Requires AutoHotkey v2.0

class SoundManager {
    static PlayTap() {
        if !AppConfig.playTapSoundEnabled
            return

        tapPath := this.FindSoundFile("tap.wav")
        if (tapPath != "" && FileExist(tapPath)) {
            SoundPlay(tapPath)
            return
        }

        navSound := A_WinDir "\Media\Windows Navigation Start.wav"
        if FileExist(navSound) {
            SoundPlay(navSound)
        } else {
            SoundPlay("*-1")
        }
    }

    static FindSoundFile(name) {
        if FileExist(A_ScriptDir "\" name)
            return A_ScriptDir "\" name
        if FileExist(A_ScriptDir "\..\" name)
            return A_ScriptDir "\..\" name
        return ""
    }
}
