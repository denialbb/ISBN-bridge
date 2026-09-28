"""Audio playback manager for tap feedback."""

from pathlib import Path
import shutil
import subprocess
from typing import Optional
from client_linux.config import AppConfig
from client_linux.logger import Logger


class SoundManager:
    _players = ["pw-play", "paplay", "aplay", "mpv"]

    @classmethod
    def find_sound_file(cls, name: str) -> Optional[Path]:
        if not name:
            return None

        # Direct path check
        p = Path(name)
        if p.is_file():
            return p.resolve()

        # Check candidate directories
        repo_root = Path(__file__).resolve().parent.parent.parent.parent
        candidates = [
            Path(name),
            repo_root / name,
            repo_root / "client" / name,
            repo_root / "client" / "sounds" / Path(name).name,
            Path.cwd() / name,
            Path.cwd() / "client" / name,
        ]
        for c in candidates:
            if c.is_file():
                return c.resolve()

        return None

    @classmethod
    def play_sound_file(cls, name: str) -> None:
        sound_path = cls.find_sound_file(name)
        if not sound_path:
            Logger.log(f"Sound file not found: {name}")
            return

        for player in cls._players:
            cmd_path = shutil.which(player)
            if cmd_path:
                try:
                    args = [cmd_path]
                    if player == "mpv":
                        args.extend(["--no-video", "--really-quiet"])
                    args.append(str(sound_path))
                    subprocess.Popen(
                        args,
                        stdout=subprocess.DEVNULL,
                        stderr=subprocess.DEVNULL,
                    )
                    return
                except Exception as e:
                    Logger.log(f"Failed to play sound with {player}: {e}")

    @classmethod
    def play_tap(cls, config: AppConfig) -> None:
        if not config.play_tap_sound:
            return
        sound_file = config.sound_file or "sounds/tap.wav"
        cls.play_sound_file(sound_file)

    @classmethod
    def set_sound(cls, config: AppConfig, filename: str) -> None:
        config.sound_file = filename
        config.save("AutoPaste", "sound_file", filename)
        cls.play_sound_file(filename)
