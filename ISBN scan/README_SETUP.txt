================================================================================
BIBLIOS ISBN AUTO-PASTE - SETUP & TROUBLESHOOTING GUIDE
================================================================================

1. FILES OVERVIEW
--------------------------------------------------------------------------------
- "ISBN paste.ahk"     : Main server script listening on port 8765.
- "Generate Token.ahk" : Generates a secure token, saves to token.txt, and copies to clipboard.
- "token.txt"          : Contains the current shared secret token.
- "biblios-debug.log"  : Log file for checking incoming requests and debugging.


2. SETTING UP ON A NEW PC
--------------------------------------------------------------------------------
Step 1: Install AutoHotkey v2 (https://www.autohotkey.com/).
Step 2: Copy this folder ("AutoHotkey") to the new PC.
Step 3: Run "Generate Token.ahk" to create a token (or keep your existing token.txt).
Step 4: Find the PC's local IP address (see commands below).
Step 5: Run "ISBN paste.ahk".
        When Windows Firewall prompts, tick BOTH Private and Public, then Allow.
Step 6: Update the iOS Shortcut URL with the new PC's IP address.


3. IOS SHORTCUT CONFIGURATION
--------------------------------------------------------------------------------
Action: "Get Contents of URL"
- URL          : http://<PC_IP>:8765/isbn     <-- IMPORTANT: use colon (:), not slash (/)!
- Method       : POST
- Headers      : Key   : Biblios-Token
                 Value : <token from token.txt>
- Request Body : File
- File         : Shortcut Input (or your Text variable)


4. USEFUL POWERSHELL COMMANDS
--------------------------------------------------------------------------------
* Find your PC's local IPv4 address:
  (Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias "Wi-Fi*").IPAddress
  (or simply run: ipconfig)

* Test the server locally from PowerShell:
  $token = Get-Content .\token.txt
  Invoke-RestMethod -Uri "http://127.0.0.1:8765/isbn" -Method POST -Headers @{ "Biblios-Token" = $token } -Body "9780306406157"

* Check if port 8765 is actively listening:
  Get-NetTCPConnection -LocalPort 8765

* Add Windows Firewall rule (Run PowerShell as Administrator):
  New-NetFirewallRule -DisplayName "Biblios Server" -Direction Inbound -LocalPort 8765 -Protocol TCP -Action Allow

* Set current Wi-Fi network profile to "Private" (Run as Administrator):
  Set-NetConnectionProfile -InterfaceAlias "Wi-Fi" -NetworkCategory Private


5. COMMON ISSUES & CHECKLIST
--------------------------------------------------------------------------------
[ ] Timeout on iOS?
    - Check the URL: must be http://<IP>:8765/isbn with a colon before 8765.
    - Check that iPhone is connected to the SAME Wi-Fi network (not 4G/5G, not Guest Wi-Fi).
    - Check iOS permission: Settings > Privacy & Security > Local Network > allow "Shortcuts".
    - Check Windows Firewall: ensure port 8765 or AutoHotkey64.exe is allowed.

[ ] 401 Unauthorized?
    - Check that the header name is exactly: Biblios-Token
    - Verify token in iOS Shortcut matches token.txt.

[ ] 400 Bad Request / Invalid ISBN?
    - Ensure the text shared contains a valid ISBN-10 or ISBN-13 (checksum verified).

[ ] Paste does not trigger when clicking?
    - Check TARGET_TAB_TITLE in "ISBN paste.ahk" (default: "hardcover").
    - The browser tab title must contain this word (case-insensitive).
    - Left-click inside a text input box (where cursor turns into an I-beam).
================================================================================
