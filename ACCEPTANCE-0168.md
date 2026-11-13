# Runtime 0.1.68 Native Capabilities

## Behavior

- Native market supplies MCP definitions; Runtime applies ACU library names and
  assignments rather than rebuilding protocol launchers. Legacy owned loopback
  tools migrate to stdio, preserving user options and unrelated MCPs.
- Native model catalog supplies vision and image candidates. No per-model Sol
  override or local profile gating. Native automatic choices stay; when no
  automatic choice exists, a native candidate supplies the fallback. Explicit
  disabled/user choices stay intact.
- Native provider list/refresh replaces direct model-list/secret extraction and
  independent background health polling. Saved-source refresh persists the
  native media catalog omitted by the add-form chat-only response.
- Native keyless search replaces bundled uv/Python/DuckDuckGo provisioning.
  Legacy default search assignments detach only after native replacement is
  read back; customized/disabled sources remain untouched.
- Model settings use native field-level CLI commands. Validate both picks before
  writes. Setup can reveal recommendations but never hides other models.
- Windows quiet Codex configuration transaction remains unchanged.

## Acceptance

- Go tests, race tests and vet pass. Native Linux CLI v0.1.3940 verified against
  official SHA-256. Four-Agent integration checks Codex, Claude, Hermes and
  WorkBuddy config, repeat idempotence, external MCP preservation, model-only
  saves and real stdio initialize/tools-list for all three tools.
- Live isolated native MCP acceptance: image analysis correctly read LEFT,
  RIGHT, FF9A518F and red/blue shapes; native keyless search returned public
  results; web_fetch read example.com; native automatic image selection saved
  a 1,087,458-byte PNG. No Runtime-specific model override was used.
- Original test credential intentionally exposes chat models only. Independent
  bounded acceptance Key exposed native image candidates; revoked afterward,
  verified reuse rejected with HTTP 401. No existing Key permissions changed.
- Preference/settings, vector graph, prices and install/update browser suites
  pass both themes and desktop/mobile layouts. Recommendation browser suite
  covers fresh/existing WorkBuddy without hiding existing model entries.
- Live evidence: /tmp/acu-native-reuse/live (private isolated profile); browser
  screenshots: /tmp/acu-native-reuse-browser/output.

## Release Boundary

Unpublished 0.1.67 pipeline was canceled after finding a missing provider-refresh
fixture in the Windows native login smoke test. 0.1.68 includes that fixture;
the canceled tag is retained for traceability and was not a public release.

Only Runtime desktop packages are published. No Router, Console or Private ACU
application restart, schema or environment change. Three-platform release CI
retains native desktop, official installation, installer upgrade and packaged
kernel checks. Local Linux MCP execution is not a user Windows GUI acceptance.

Native protocol metadata may still identify Magpie; library/plugin names are ACU.
The shipped kernel path is stable across package replacement. Cross-computer
configuration sync involving a removed ACU-named kernel executable is not covered
by this release's native self-healing guarantee.

## Published

Workflow 43510144958 passed Windows x64, macOS arm64 and macOS Intel, then
published v0.1.68 at 2026-11-13 21:35:08 UTC from d268aa8. Windows native login,
renewal, quiet Codex transaction, official first installation, old installer
upgrade and migrated native MCP compatibility all passed.

The official website resolver returns 0.1.68. Full accelerated downloads matched
GitHub asset sizes and SHA-256 on all platforms:

- Windows: 51,913,728 bytes, previously 67,160,064 bytes (about 23% smaller).
- Apple Silicon DMG: 49,492,650 bytes.
- Intel Mac DMG: 54,186,212 bytes.

Download evidence: /tmp/acu-runtime-speed-verification.json. Live native MCP
usage was recorded for GPT-6.1 Sol and Sunburst image generation; the latter
saved and visually verified the requested red square/blue circle image.
