import http.client
import subprocess
import time
import urllib.request
import hashlib
from datetime import datetime

AHK_EXE = r"C:\Users\DanyB\AppData\Local\Programs\AutoHotkey\v2\AutoHotkey64.exe"
AHK_SCRIPT = r"C:\Users\DanyB\OneDrive\Documenti\AutoHotkey\client\main.ahk"
GO_EXE = r"C:\Users\DanyB\OneDrive\Documenti\AutoHotkey\bin\biblios-server.exe"

def test_full_pipeline():
    print("1. Starting AutoHotkey client...")
    ahk_proc = subprocess.Popen([AHK_EXE, AHK_SCRIPT])
    
    print("2. Starting Go server...")
    go_proc = subprocess.Popen([GO_EXE])
    
    time.sleep(1.2)
    
    try:
        # AHK Health: POST /qr/show
        print("3. Testing AHK POST /qr/show...")
        req = urllib.request.Request("http://127.0.0.1:8766/qr/show", data=b"", method="POST")
        with urllib.request.urlopen(req, timeout=3) as resp:
            assert resp.status == 200, f"Expected 200, got {resp.status}"
            print("   -> AHK QR Show OK")

        time.sleep(0.3)

        # AHK Health: POST /qr/hide
        print("4. Testing AHK POST /qr/hide...")
        req = urllib.request.Request("http://127.0.0.1:8766/qr/hide", data=b"", method="POST")
        with urllib.request.urlopen(req, timeout=3) as resp:
            assert resp.status == 200, f"Expected 200, got {resp.status}"
            print("   -> AHK QR Hide OK")

        # AHK Direct Paste: POST /paste
        print("5. Testing AHK direct POST /paste...")
        isbn = "9780306406157"
        req = urllib.request.Request("http://127.0.0.1:8766/paste", data=isbn.encode("utf-8"), method="POST")
        with urllib.request.urlopen(req, timeout=3) as resp:
            assert resp.status == 200, f"Expected 200, got {resp.status}"
            print("   -> AHK Direct Paste OK")

        # Go Server: GET /health
        print("6. Testing Go server GET /health...")
        with urllib.request.urlopen("http://127.0.0.1:8765/health", timeout=3) as resp:
            assert resp.status == 200
            print("   -> Go Server Health OK")

        # Read active token
        with open("token.txt", "r", encoding="utf-8") as f:
            token = f.read().strip()
        print(f"   -> Read active token: {token[:8]}...")

        # Go Server: Authenticated POST /isbn (with forward to AutoHotkey)
        print("7. Testing End-to-End iOS scan request: POST /isbn -> Go -> AHK...")
        ts = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        auth_raw = f"{isbn}|{ts}|{token}".encode("utf-8")
        sig = hashlib.sha256(auth_raw).hexdigest()

        req = urllib.request.Request(
            "http://127.0.0.1:8765/isbn",
            data=isbn.encode("utf-8"),
            headers={
                "Authorization": f"Bearer {sig}",
                "Timestamp": ts,
                "Content-Type": "text/plain; charset=utf-8"
            },
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=5) as resp:
            body = resp.read().decode("utf-8")
            assert resp.status == 200, f"Expected 200, got {resp.status}"
            assert body == "OK", f"Expected body 'OK', got {body}"
            print(f"   -> iOS Scan -> Go Auth -> AHK Forward: SUCCESS! Body: {body}")

        print("\n==================================================")
        print("ALL TESTS PASSED! Full round-trip integration verified!")
        print("==================================================")

    finally:
        print("Cleaning up processes...")
        try:
            ahk_proc.terminate()
            ahk_proc.wait(timeout=1)
        except Exception:
            ahk_proc.kill()

        try:
            go_proc.terminate()
            go_proc.wait(timeout=1)
        except Exception:
            go_proc.kill()

if __name__ == "__main__":
    test_full_pipeline()
