#Requires AutoHotkey v2.0

class SoundManager {
    static PlayTap() {
        if !AppConfig.playTapSoundEnabled
            return

        soundFile := (AppConfig.soundFile != "") ? AppConfig.soundFile : "tap.wav"
        this.PlaySoundFile(soundFile)
    }

    static PlaySoundFile(name) {
        soundPath := this.FindSoundFile(name)
        if (soundPath != "" && FileExist(soundPath)) {
            SoundPlay(soundPath)
            return
        }

        navSound := A_WinDir "\Media\Windows Navigation Start.wav"
        if FileExist(navSound) {
            SoundPlay(navSound)
        } else {
            SoundPlay("*-1")
        }
    }

    static SetSound(filename) {
        AppConfig.soundFile := filename
        AppConfig.Save("AutoPaste", "sound_file", filename)
        this.PlaySoundFile(filename)
    }

    static FindSoundFile(name) {
        if (name = "windows_navigation" || name = "sounds/windows_navigation.wav") {
            navSound := A_WinDir "\Media\Windows Navigation Start.wav"
            if FileExist(navSound)
                return navSound
        }
        if FileExist(name)
            return name
        if FileExist(A_ScriptDir "\" name)
            return A_ScriptDir "\" name
        if FileExist(A_ScriptDir "\..\" name)
            return A_ScriptDir "\..\" name
        return ""
    }
}
