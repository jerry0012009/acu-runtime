# Runtime 0.1.80 Publication

Published 2026-11-15 at 05:52:15 UTC. Application/tag SHA:
`954d405c958648b5950f979b7deafe83f9478e2e`.

User explicitly requested a formal release to perform device testing. This release
retains Runtime 0.1.79 video/model recommendations and team preferences, adding
optional guided TapNow ZIP setup. Chrome Web Store is not required. First Load
Unpacked and subsequent Reload remain user confirmations; Key/model configuration
is automatic and receipt-confirmed. See TAPNOW.md for protocol and safety details.

## Build Evidence

Validated workflow:
https://github.com/jerry0012009/acu-runtime/actions/runs/43681870855

Windows x64, macOS arm64 and macOS amd64 jobs all succeeded on the same application
SHA. Includes native desktop tests/vet, frontend/browser regressions, released ZIP
digest checks and real MV3-to-native-host configuration/receipt on each platform.
Existing official WorkBuddy install, Windows ChatGPT install, quiet Codex routing,
old Runtime upgrade and bundled-kernel compatibility checks also passed.

The successful workflow was dispatched on the task branch and intentionally did
not run its tag-only publish job. Its exact five installer assets were downloaded,
hashed, uploaded to a draft, verified against GitHub asset digests, then published
as the latest stable release. No second build was substituted. Publishing the
release created a tag push and a redundant workflow (43682919813); cancellation
was requested so it cannot replace already verified release bytes.

| Asset | SHA-256 |
| --- | --- |
| Windows x64 Setup.exe | `721cfa0213b799e236d836520b8eea9a8ceaf4552da365adfc399d8f5551781c` |
| macOS arm64 DMG | `2690b100ca5d8692c3b7752de7550cdacac24d08c7aafc6127c9307d5b3fe93d` |
| macOS amd64 DMG | `ea1ee93cb42678415c80ea200633ea9be5aeeab8b19aedc1ba95d4dcda168c22` |
| macOS arm64 app.zip | `13b80053cf539f1830bd829b7ec9b6a91f65cd45ea31123d3aa1eed19beb6408` |
| macOS amd64 app.zip | `edd89aaf228ff3ac6cd9e2adf8b57509dcc6457db483f33d89f6613946c13bdb` |

Release: https://github.com/jerry0012009/acu-runtime/releases/tag/v0.1.80
Public SHA256SUMS download returned HTTP 200. GitHub latest and Console's public
`/api/acu-runtime/release` both returned 0.1.80 with all three download targets.

## Plugin Supply

Repository `jerry0012009/acu-tapnow` was initialized from an allowlisted source
export without old Git history, real canvas examples, old ZIPs or trial keys.
Plugin tag source: `16198038c437654475c777b86630bc2f774141e8`.
Catalog activation: `042cd288f0d95d173d06fbdecc12069d53097e8c`.
Plugin workflow 43681662398 succeeded (85 tests, audit, identity/entry-verified ZIPs).

- Companion 0.1.23, 44,702 bytes: `f8aac10e5bb6cd1d9ce4a4855091589a6bffd0abcd5d868767417a7143ac7b5e`.
- Backup 0.1.23, 188,886 bytes: `e19b87559ddae33a62e171bc0e108af806ff19b9cba420579d619ece4a4daf7a`.

Published ZIPs were downloaded and rehashed before activating the main catalog.
Runtime CI downloaded these exact assets and checked the pinned hashes.

## Remaining Device Tests

Actual personal Chrome Load Unpacked, organization policies, custom Chrome data
directories, real authorized TapNow canvas review/backup and old-ID migration are
not proven by isolated CI. No paid inference or private account data was used.
User testing of these flows is the reason for this first public release. Existing
plugin content/LLM TypeScript diagnostics remain disclosed; successful builds do
not imply a project-wide clean type check. No Windows code signing or Apple
Developer ID notarization was added. No Router/Console production deployment,
restart, database or environment change was performed.
