# Video Reading Integration

Development candidate, not a published Runtime or TapNow release.

## Reused implementation

Video parsing, frame extraction, structure inspection and detail tools come from
`claude-video-vision@1.4.0` (MIT). ACU adds installation, visual-only defaults and
the existing Library/Native Messaging adapters, not another media engine.

One-click Agent setup prepares a private Node 24.22.3 distribution, checks its
pinned SHA-256, installs exact lockfile dependencies using `npm ci --ignore-scripts`,
checks FFmpeg/FFprobe execution and assigns `acu-video-reader` through Magpie.
No system Node, shell profile, model credentials or existing custom MCP is replaced.
Subsequent setup preserves custom commands, disabled tools, pause state and assignments.
The unchanged previous bundled image guidance is upgraded to mention video reading;
user-edited guidance is preserved.

The launcher exposes four upstream read tools and forces cloud/local audio
transcription off. It does not download Whisper or send inference during setup.
Reading returns images and timestamps, not an automatic text interpretation.
An image-capable model/client or a separately verified image interpretation path
is required. Native tool trust prompts and reopening an Agent remain client-owned.

## TapNow

Companion setup also prepares the same dependencies. The existing native host
accepts a small `video-frames` request only from the Companion extension identity.
Backup is refused. A fresh request works after Runtime exits: reading uses the
prepared private installation, not the expiring credential-configuration grant.
Unpacked extension identity is not package authentication; trust the verified ZIP
and the local OS account, as in the existing native connection.

Only exact HTTPS files.tapnow.media/files.tapnow.top URLs are accepted; no cookies
or TapNow bearer token are sent. Referer is fixed to app.tapnow.ai. Redirects,
private DNS addresses, oversized downloads and non-video responses are refused.
Downloaded video is temporary, at most 100 MiB and 30 minutes. Upstream MCP
provides approximately 12 whole-video samples, 640px wide; native results are
bounded below Chrome's 1 MiB host-response limit. At most three videos are read
per review. Omitted/failed material and lack of audio are explicit.

Companion combines samples with its existing image/text review and creative-memory
service. It distinguishes upstream references from unbound existing output.
Changed video URLs/roles/node identity invalidate late results. It does not prove
that existing output belongs to the current edited prompt, auto-regenerate video,
intercept TapNow generation, or independently train preferences from its own critique.

## Verification

- Runtime Go tests, race tests, vet and Windows test-binary cross-compilation.
- Runtime frontend tests and light/dark 390px/1440px browser screenshots.
- Pinned Magpie 0.1.4096: all four Agent writers, repeated configuration,
  preserved custom MCP/model choices, and four real MCP tool lists passed.
- Opt-in fresh-HOME installation and actual upstream MCP reading of synthetic MOV:
  `ACU_VIDEO_READING_SMOKE=1 go test -run '^TestVideoReadingRealMCP$' -v -timeout 7m .`
- Actual Chromium MV3/native-host configuration roundtrip and denied unsafe video
  requests, plus plugin unit tests and both extension builds.

No real private TapNow canvas, paid vision model quality, ProRes/HDR/HEVC matrix,
Windows/macOS dependency execution, or every Agent's image-tool-result handling
is inferred from Linux fixtures. Full plugin strict typecheck has pre-existing
diagnostics; the new video module's focused strict typecheck passes. Public ZIP/version
catalog activation and cross-platform Runtime publication are separate release work.
