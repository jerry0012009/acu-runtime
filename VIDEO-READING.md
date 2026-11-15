# Standard external video MCP

Runtime 0.1.88 uses unchanged official Magpie and the independent self-contained
`jerry0012009/acu-video-mcp` 0.1.0 release. No Magpie source patch, private video
API, alternate kernel update feed, runtime npm installer or Runtime video decoder.

Explicit one-click setup downloads the platform tool executable to the current
user's versioned tools directory and verifies the pinned release SHA-256. The
tool's `--prepare` unpacks its embedded dependencies; `--register-native` registers
its own `com.acucompute.video` host for Companion only. Runtime writes a normal
stdio MCP definition through Magpie's existing Library API. Existing assignments,
custom launchers, hidden state and disabled tools remain. The old unpublished
Runtime launcher is migrated only when its exact owned command matches.

The independent executable bundles Node 24.22.3, unchanged upstream
claude-video-vision 1.4.0, FFmpeg and FFprobe. No user-installed Node/FFmpeg or
provider key is required for frame extraction. Media parsing and sampling remain
upstream-owned; the adapter enforces visual-only mode. Frames still require an
image-capable model/client or verified image interpretation path.

TapNow uses its own `com.acucompute.video` Native Messaging request directly.
Runtime and the Magpie window need not proxy media. The independent host accepts
only the Companion origin, exact HTTPS TapNow media hosts and bounded requests;
Backup and other origins are refused. Redirects/private DNS addresses are refused,
downloads are temporary, max 100 MiB/30 minutes, and sample results stay below
Chrome's 1 MiB reply limit. Audio is not analyzed. Existing output is not assumed
to match an edited prompt, and suggestions never trigger generation or learning.

The independent release's Windows x64, Linux x64 and both Mac architecture native
CI builds/race tests/vet/MOV self-tests passed before its hashes were activated.
Runtime/plugin tests, official-kernel configuration and Native Messaging checks
are tracked in release acceptance. This is not exhaustive video defect detection
or a claim of paid-model quality acceptance. Chrome's first extension load and
Agent trust remain user confirmations. Official kernel updates remain independent.
