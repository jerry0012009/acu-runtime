# 0.1.57 update and crash-repair acceptance

User feedback on 2026-11-13 (Asia/Shanghai) confirms the 0.1.55 repair fixes the Windows desktop crash loop. This release merges that verified route transaction into the newer 0.1.56 release, preserves preference editing drafts, and pins official Magpie 0.1.3940 assets.

Local checks: 114 JavaScript tests; preference browser layouts and draft-edit regression; isolated official native kernel and installed Codex RPC with rollback/cancellation/disconnect. Native configuration measured 6.8 seconds in the isolated fixture; no claim is made for all network or user environments. Repeated startup policy commands are skipped only when current settings are readable and already safe; changed or invalid settings trigger native commands again. Probe shutdown ends only its own job.

Release gates include Go and native desktop suites/vet, installer payload pins and actual verify-only execution, official ChatGPT/WorkBuddy first installation, native WebView session/restore checks, quiet Codex routing, upgrade from the published 0.1.56 installer, matching serving kernel and unchanged existing Codex config/auth, kernel API compatibility, and Apple Silicon/Intel native packaging. All platform builds must pass before a complete release becomes latest.

Windows x64 supports in-app Runtime updates; macOS receives the same compatible pinned kernel in DMG/app.zip and currently requires replacing the app. Custom external gateways are not stopped automatically. Unknown kernel state blocks Codex writes. Full GUI tasks and real accounts on arbitrary devices cannot be proven by these checks.
