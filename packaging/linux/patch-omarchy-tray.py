#!/usr/bin/env python3
"""Route left-click on the ISBN Bridge tray icon to Show-QR.

Why this exists: ISBN Bridge uses libayatana-appindicator, whose SNI
exporter activates the secondary target only on SecondaryActivate
(middle click) and swallows a primary Activate (left click) in its
`bus_method_call` "unknown method" branch. AyatanaAppIndicator3.Indicator
exposes zero GObject signals, so the app cannot intercept left click
itself. Omarchy's bar widget (Tray.qml) sends `activate()` on left
click, which therefore dies inside the C library.

This patch adds an item-scoped branch: left click on the `isbn-bridge`
item fires `secondaryActivate()` (Show QR, like middle click). All other
icons keep the stock behavior.

Target file is system-owned and may be overwritten by Omarchy updates;
re-run with sudo to re-apply. Run with `sudo`.

Usage:
    sudo ./patch-omarchy-tray.py          # apply (backs up to *.bak-isbn-bridge)
    ./patch-omarchy-tray.py --check       # verify applied (no root needed)
    sudo ./patch-omarchy-tray.py --revert # restore backup
Then: omarchy-restart-shell
"""

import sys
from pathlib import Path

TRAY_QML = Path("/usr/share/omarchy/shell/plugins/bar/widgets/Tray.qml")
BACKUP_SUFFIX = ".bak-isbn-bridge"

OLD = """        } else if (trayItemRoot.modelData.onlyMenu) {
          trayItemRoot.displayMenu(mouse)
        } else {
          trayItemRoot.modelData.activate()
        }"""

NEW = """        } else if (trayItemRoot.modelData.onlyMenu) {
          trayItemRoot.displayMenu(mouse)
        } else if (String(trayItemRoot.modelData.id) === "isbn-bridge") {
          // ISBN Bridge uses libayatana-appindicator, which swallows the
          // primary Activate (left click) internally: route it to the
          // secondary target (Show QR), like a middle click.
          trayItemRoot.modelData.secondaryActivate()
        } else {
          trayItemRoot.modelData.activate()
        }"""


def main(argv: list[str]) -> int:
    if not TRAY_QML.is_file():
        print(f"not found: {TRAY_QML}", file=sys.stderr)
        return 1
    text = TRAY_QML.read_text()

    if "--check" in argv:
        print("applied" if NEW in text else "not applied")
        return 0 if NEW in text else 1

    if "--revert" in argv:
        backup = TRAY_QML.with_name(TRAY_QML.name + BACKUP_SUFFIX)
        if not backup.is_file():
            print("no backup found, nothing to revert", file=sys.stderr)
            return 1
        TRAY_QML.write_text(backup.read_text())
        print(f"reverted from {backup}")
        return 0

    if NEW in text:
        print("already applied")
        return 0
    if text.count(OLD) != 1:
        print("error: expected click block not found exactly once; "
              "Omarchy may have changed Tray.qml", file=sys.stderr)
        return 1
    backup = TRAY_QML.with_name(TRAY_QML.name + BACKUP_SUFFIX)
    if not backup.is_file():
        backup.write_text(text)
        print(f"backup written to {backup}")
    TRAY_QML.write_text(text.replace(OLD, NEW))
    print("patched: left click on isbn-bridge now fires secondaryActivate (Show QR)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
