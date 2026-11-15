# External Audio Reading

Runtime 0.1.93 installs verified acu-video-mcp 0.2.3 and supplies the current
ACU connection through official Magpie Library stdio environment. It has no
audio decoder, chunk planner, speech engine, model proxy or Magpie patch.
Official yetone/magpie release downloads and update ownership remain unchanged.

Recommended tool setup configures auxiliary gemini-3.5-flash-lite when available
without changing the main conversation/vision model. Tests cover standard Library
environment credential propagation, preserving custom commands, and upgrading
only owned versioned executable paths while keeping paused/disabled choices.
TapNow Companion 0.1.29 uses its existing local key through the fixed-identity
native channel; Backup remains separate and has no key/audio capability.

Local Go tests/race/vet and 132 frontend behavior tests passed. TapNow's 94
tests and both ZIP builds passed; TypeScript baseline comparison added no errors
(seven existing diagnostics remain). The independent MCP's four platform jobs
passed real MOV conversion/chunking tests and self-tests. Downloaded Linux MCP
binary public speech and non-speech acceptance and acknowledged costs are in
acu-video-mcp/ACCEPTANCE-023.md. Private canvases and end-user WorkBuddy UI are
not inferred from those fixtures.

Upgrade Runtime, reapply recommended tools / TapNow connection once to replace
the old pinned external executable, reload Companion in Chrome and refresh the
TapNow page. Browser extension and Agent trust confirmations remain manual.
No Google key, local Whisper model or manual Node/FFmpeg installation is needed.
