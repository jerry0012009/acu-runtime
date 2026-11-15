# Runtime 0.1.90 external video reading

Uses released acu-video-mcp 0.1.0 self-contained executables and TapNow 0.1.27.
Official Magpie v0.1.4248 binaries and source/update channel remain unchanged.
There is no fork, kernel patch, private /api/library/video API, Runtime npm or
FFmpeg bootstrap, or Runtime media decoding subprocess.

The independent tool's Windows x64, macOS x64/arm64 and Linux x64 release jobs
passed native builds, race tests, vet and synthetic MOV reading before hashes
were pinned. On the development host, actual unchanged official Magpie and this
Runtime passed four Agent configuration writers, repeat/no duplicates, existing
MCP preservation, model-only writes preserving Agent files, and four real MCP
tool lists including the independent video MCP. Setup downloaded the released
tool, verified its SHA-256, prepared embedded dependencies and registered its
separate current-user Chrome Native Messaging host. No model call was sent.

Runtime race tests and vet passed, 132 frontend tests passed, light/dark mobile
and desktop capability checks passed. Companion 92 tests, zero-vulnerability
audit, both ZIP builds/identity/entry checks and publication succeeded. Existing
Chromium credential/native-host smoke passed without cloud inference.

Runtime 0.1.90 published at 2026-11-15 12:11:29 UTC from
`524dcaaef40eb445ad9e92356203e0433d2d470f`, workflow
[43711527439](https://github.com/jerry0012009/acu-runtime/actions/runs/43711527439).
Windows x64 and macOS arm64/amd64 jobs, packaged kernel/tool compatibility,
native clipboard, released TapNow ZIP/native connection checks and publication
all passed. The release contains five desktop packages, INSTALL.md and
SHA256SUMS.txt. Later closeout changes are documentation only.

The user subsequently reported successful TapNow installation on their device;
this is user-reported acceptance, not an independently observed device session.
Real MOV inputs and model review quality still need device acceptance.
No private live-canvas/paid-model acceptance, exhaustive video
defect detection or audio analysis is claimed. Missing/omitted video evidence is
reported; existing results are not bound to edited prompts without provenance.
