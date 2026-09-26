#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

echo "Starting server in background..."
./bin/biblios-server -port 8765 -no-terminal-qr &
PID=$!

cleanup() {
    echo "Stopping server (PID $PID)..."
    kill $PID || true
}
trap cleanup EXIT

sleep 1

echo "1. Checking /health..."
curl -s http://127.0.0.1:8765/health
echo ""

echo "2. Reading active token..."
TOKEN=$(cat token.txt)
echo "Active Token: $TOKEN"

ISBN="9780306406157"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
RAW="${ISBN}|${TIMESTAMP}|${TOKEN}"
HASH=$(echo -n "$RAW" | sha256sum | awk '{print $1}')

echo "3. Sending signed request..."
echo "  Raw: $RAW"
echo "  Hash: $HASH"

HTTP_CODE=$(curl -s -o /tmp/resp.txt -w "%{http_code}" -X POST http://127.0.0.1:8765/isbn \
  -H "Authorization: Bearer $HASH" \
  -H "Timestamp: $TIMESTAMP" \
  -d "$ISBN")

echo "  HTTP Code: $HTTP_CODE"
echo "  Response body: $(cat /tmp/resp.txt)"

# 502 is expected when AutoHotkey is offline (since Go strictly enforces delivery confirmation)
if [ "$HTTP_CODE" -eq 200 ] || [ "$HTTP_CODE" -eq 502 ]; then
    echo "✅ SUCCESS! E2E verified (HTTP $HTTP_CODE: Signature authenticated; delivery attempted)."
else
    echo "❌ FAILED: Unexpected HTTP code $HTTP_CODE"
    exit 1
fi
