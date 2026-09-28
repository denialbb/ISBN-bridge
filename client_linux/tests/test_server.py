import tempfile
import urllib.request
import urllib.error
import pytest
from client_linux.config import AppConfig
from client_linux.paste import PasteEngine
from client_linux.server import HttpListener

@pytest.fixture
def test_server():
    config = AppConfig()
    config.http_port = 18766
    config.auto_paste_on_hover = False
    engine = PasteEngine(config)

    with tempfile.NamedTemporaryFile("w+", delete=False) as f:
        f.write("my-super-secret-token\n")
        secret_file = f.name

    listener = HttpListener(config, engine, secret_file_path=secret_file)
    listener.start()

    yield listener, engine, "my-super-secret-token"

    listener.shutdown()
    import os
    if os.path.exists(secret_file):
        os.remove(secret_file)

def test_missing_secret_returns_403(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    req = urllib.request.Request(url, data=b"9780306406157", method="POST")
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req)
    assert exc_info.value.code == 403

def test_valid_paste_arms_engine(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    headers = {"X-ISBN-Bridge-Local": secret}
    req = urllib.request.Request(url, data=b"9780306406157", headers=headers, method="POST")
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
        assert resp.read() == b"OK"
    assert engine.pending_isbn == "9780306406157"

def test_invalid_isbn_returns_400(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    headers = {"X-ISBN-Bridge-Local": secret}
    req = urllib.request.Request(url, data=b"invalid-isbn-123", headers=headers, method="POST")
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req)
    assert exc_info.value.code == 400

def test_trigger_endpoint(test_server):
    listener, engine, secret = test_server
    engine.pending_isbn = "9780306406157"
    url = f"http://127.0.0.1:18766/trigger"
    headers = {"X-ISBN-Bridge-Local": secret}
    req = urllib.request.Request(url, headers=headers, method="POST")
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200

def test_wrong_secret_returns_403(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    headers = {"X-ISBN-Bridge-Local": "wrong-secret-token"}
    req = urllib.request.Request(url, data=b"9780306406157", headers=headers, method="POST")
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req)
    assert exc_info.value.code == 403

def test_malformed_content_length_returns_400(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    headers = {"X-ISBN-Bridge-Local": secret, "Content-Length": "invalid"}
    req = urllib.request.Request(url, data=b"9780306406157", headers=headers, method="POST")
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req)
    assert exc_info.value.code == 400

def test_negative_content_length_returns_400(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    headers = {"X-ISBN-Bridge-Local": secret, "Content-Length": "-1"}
    req = urllib.request.Request(url, data=b"9780306406157", headers=headers, method="POST")
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req)
    assert exc_info.value.code == 400

def test_oversized_content_length_returns_413(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/paste"
    headers = {"X-ISBN-Bridge-Local": secret, "Content-Length": "20000"}
    req = urllib.request.Request(url, data=b"x" * 20000, headers=headers, method="POST")
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req)
    assert exc_info.value.code == 413

def test_status_endpoint(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/status"
    req = urllib.request.Request(url, method="GET")
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
        import json
        data = json.loads(resp.read().decode())
        assert data["status"] == "ok"

def test_qr_show_endpoint(test_server):
    listener, engine, secret = test_server
    url = f"http://127.0.0.1:18766/qr/show"
    headers = {"X-ISBN-Bridge-Local": secret}
    req = urllib.request.Request(url, headers=headers, method="POST")
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
        assert resp.read() == b"QR shown"

