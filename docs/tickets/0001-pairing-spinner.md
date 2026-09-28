# Ticket: Streamline pairing — `/pair` spinner interstitial (iOS + Android)

**Status:** implemented (landed in `cfd4fbe`, see collision note) · **Scope:** server pairing page + docs · **Out of scope:** QR payload scheme change, HTTP Shortcuts deep-link import

## Problem

Pairing takes 4 perceived steps: scan QR → Safari `/pair` page → tap button → Pair shortcut → Scanner.
The Safari page looks like a decision screen, so users hesitate. The Apple system
"Open in Shortcuts?" confirmation cannot be removed — but our page's button tap can.

Constraints (confirmed with reporter):

- Keep `https /pair` QR payload. No `shortcuts://` QR — server-side scan detection
  (desktop QR auto-hide) must keep working.
- Both iOS and Android. Android Binary Eye handoff still untested — don't over-invest there.
- Pair shortcut already chains to scanner but lingers (waits on scanner close instead of exiting).

## Change 1 — iOS `/pair` becomes a loading interstitial (server)

File: `pkg/server/server.go` (`handlePair`).

- Replace the two-button card with a fullscreen spinner: title "Pairing… opening Shortcuts",
  sub reuses `pair_ios_sub` ("If Safari asks, tap **Open in Shortcuts**…").
- Keep 400ms auto-redirect to primary `shortcuts://run-shortcut?name=Pair ISBN Bridge`.
  Add one fallback timer (~1.5s, gated on `document.visibilityState == 'visible'`)
  targeting alt name `ISBN Bridge Pair`.
- Demote manual actions into `<details>"Didn't open automatically?"</details>`:
  primary link, alt link, copy-config button. No primary CTA on first paint.
- Add lang keys (e.g. `pair_spin_title`, `pair_fallback_summary`) to
  `pkg/server/lang/en.json` + `it.json`. External `lang/` override mechanism unchanged.
- Invariants: token stays in query/body as today; zero log leakage;
  `isMobileUserAgent → HideQR` path untouched.

## Change 2 — Pair shortcut exits after launching scanner (Shortcuts config + docs)

No server code. In the Pair shortcut: `Run Shortcut [Scanner]` → **Wait Until Finished OFF**,
followed by `Exit Shortcut`.

File: `docs/SHORTCUTS.md` — make this normative (currently "optional & recommended").

## Change 3 — Android `/pair`: one-tap copy + split values (server, small)

Same spinner shell; action = one big `Tap to copy` (auto-copy on load is blocked in Chrome
without a gesture — do not rely on it) + existing `Download config` fallback. Add two
separate tappable copy rows for `url` and `token` (current whole-JSON copy forces manual
splitting into `isbn_server_url` / `isbn_token` per `mobile/android/SETUP.md`).
Keep `?format=json` untouched. No `http-shortcuts://` import work — unverified,
spike separately after Binary Eye test.

## Files

`pkg/server/server.go` (handlePair only) · `pkg/server/lang/en.json` ·
`pkg/server/lang/it.json` · `pkg/server/server_test.go` (/pair assertions: spinner markup,
both URIs, no primary-button-first) · `docs/SHORTCUTS.md` · optionally `mobile/android/SETUP.md`.

## Acceptance

- [ ] iOS: scan → brief spinner → 1 Apple system tap → Pair exits immediately → scanner open.
- [ ] Android: scan → 1 tap copies usable values; download fallback works.
- [ ] Desktop QR auto-hides on scan on both platforms.
- [ ] `go test ./pkg/server/` green; no token/secret material in logs.

## Collision note

Another agent is active in the codebase (GTK3 client work). Their `server.go` hunks were
log-hygiene + renames only, disjoint from `handlePair` — no code conflict. Heads-up:
their commit `cfd4fbe` (17:08 UTC) swept this ticket's uncommitted changes in under the
`feat(linux)` message. Code + tests verified green at HEAD.
