// ISBN Bridge — HTTP Shortcuts (Android) pre-execution script.
//
// Install: HTTP Shortcuts + Binary Eye (barcode scanner backend).
// Pairing: open the PC's /pair page in Chrome, copy `url` and `token`
// into global variables `isbn_server_url` and `isbn_token` (secret).
// Shortcut: POST to {isbn_server_url}/isbn, plain-text body {isbn_current},
// headers Authorization: Bearer {isbn_sig} and Timestamp: {isbn_ts}.
// Attach this as the "Run before execution" script. Tested against the
// Go verifier (see mobile/android/SETUP.md).
const isbn = scanBarcode();
if (isbn == null) abort();
const now = new Date();
const p = (n) => (n < 10 ? '0' : '') + n;
const ts = now.getFullYear() + '-' + p(now.getMonth()+1) + '-' + p(now.getDate()) +
           ' ' + p(now.getHours()) + ':' + p(now.getMinutes()) + ':' + p(now.getSeconds());
setVariable('isbn_current', isbn);
setVariable('isbn_ts', ts);
setVariable('isbn_sig', hash('SHA-256', isbn + '|' + ts + '|' + getVariable('isbn_token')));
vibrate('tick');
enqueueShortcut('ISBN Bridge Scanner', null, 800); // continuous loop
