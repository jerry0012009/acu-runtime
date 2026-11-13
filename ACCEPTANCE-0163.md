# Runtime 0.1.63 Accelerated Updates

Windows Runtime installer downloads now prefer `gh-proxy.com`, fall back to the
official GitHub URL and finally `ghfast.top`. Version metadata still prefers the
official GitHub API, with its existing proxy fallback. The installer URL must
match the official repository, version, platform and asset name; the installer
must match the expected SHA-256 before execution. No ACU Key or user session is
sent in these downloads. Existing clients need one upgrade to obtain this source
ordering; this release cannot modify their already-installed update code.

The original first-source 45-second budget would interrupt a 64MB installer at
the owner's observed 1.36MB/s mainland download speed. Proxies now receive five
minutes for the complete download; direct GitHub retains the 45-second budget.
Response headers remain bounded by the existing 12-second timeout. There is no
promise of constant speed across all mainland networks or future proxy uptime.

Local validation passed `go test -race ./...`, `go vet ./...`, 114 JavaScript
tests, and explicit download tests for proxy priority, correct full-download
deadline, corrupt mirror fallback and verified final output. Existing fallback
tests cover HTTP503, corruption, alternate source order and cancellation.

The matching website change is independently released as acu-frontend
`db34fe55ed96f0e526498257e29a4acebcd66e65`, blue/green manifest
`bg-20261113T104421-372a2072`, status completed. All three website installers
default to the proxy and retain an explicit GitHub-source link. Desktop/mobile
browser checks, range downloads, login continuity and real three-protocol
terminal/unique-settlement acceptance passed. No Router, Private, Nginx or
environment changes were necessary. macOS still uses manual app replacement.

Release CI: `43410439866`, application commit
`9b56ca9805916470b191a635709aa7d79c8e990f`. First Windows attempt passed normal
native login and restart restoration but timed out in the existing bipolar
login fixture at `#quick-key`. Both Mac jobs passed. The failed Windows job was
rerun without removing or relaxing any tests, and failed at the same selector.
The 0.1.63 tag was not published. Diagnostic-only `97caae9` was not merged;
workflow `43413324108` was canceled once the newer 0.1.64 workflow passed the
unchanged native-login tests. Concurrent 0.1.64 commit `51413a5` preserves all
accelerated-update code and supersedes this attempted release. Do not publish
the older 0.1.63 tag as latest.

Final successor publication: 0.1.64 workflow `43413068273` passed all three native
platforms and publication. The website resolver returns 0.1.64. Full downloads
of its Windows and both Mac installers through the website's accelerated
redirect matched official GitHub asset sizes and SHA-256 digests. Evidence:
`/tmp/acu-runtime-speed-verification.json` and Router operations report
`docs/operations/2026-11-13-runtime-download-acceleration.md`.
