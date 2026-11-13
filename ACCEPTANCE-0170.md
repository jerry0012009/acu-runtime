# Runtime 0.1.70 Display Branding

## Change Boundary

Official Codex configuration reference defines model_providers.<id>.name as a
display name. The shipped Magpie v0.1.3940 writes magpie in that field. Runtime
now changes only that string to ACU inside the existing quiet Codex configuration
transaction, before its native validation. Provider ID remains magpie: existing
thread provider identity, routes, tokens, headers and login behavior stay intact.

A pinned TOML parser locates the exact value's byte range; only that string is
replaced atomically. No whole-document serialization, background watcher,
kernel fork, proxy or desktop DOM patch. Custom names and unsupported TOML forms
remain untouched. Native later reconfiguration may restore its upstream display
name; the next Runtime configuration reapplies ACU, without a policing loop.

Embedded ordinary text/tooltips and model-option labels use ACU instead of
ACU kernel. Code, inputs, links, protocol fields and legal notices remain intact.
MCP library names already use acu-vision, acu-image and acu-search.

## Verification

- Go unit/race suite, vet and 123 frontend tests pass.
- TOML tests cover comments, quoted tables, dotted keys, idempotence, custom
  names, unrelated providers and malformed/unsupported config preservation.
- Real official kernel + installed Codex isolation verifies name=ACU persists
  after restart, selected models load, all three login choices remain intact,
  rollback/cancellation/disconnect and unrelated MCP preservation pass.
- Embedded native page browser tests pass light/dark, 390/1440 widths, dynamic
  text and tooltip branding, exact code preservation and unchanged link URLs.
- Screenshots: /tmp/acu-codex-brand-browser/output/codex-login.
- The user's actual Windows menu is not locally accessible. Native configuration
  and app-server loading are verified; no claim of remote user-device GUI test.

Only Runtime packages are published. No Router/Console/Private deployment,
schema/environment change or user account mutation.

Reference: https://developers.openai.com/codex/config-reference/

## Published

Workflow 43519005071 passed Windows x64, macOS arm64 and macOS Intel, and
published v0.1.70 from 08e9868 at 2026-11-13 22:35:54 UTC. The official site
resolver returns v0.1.70. Full downloads for all three installers matched the
GitHub asset sizes and SHA-256 digests.

Windows upgrade/download: 52,072,448 bytes, SHA-256
bda3e5815402b3286131a332962254906c8603dcb4cd64cfc025ccb94eacaf86.
Apple Silicon DMG: 50,169,719 bytes, SHA-256
fe41006beb5fea233d33921d494a28bb87b37ec99af452c57169978e491c6bff.
Intel Mac DMG: 53,930,595 bytes, SHA-256
0ed53f44b35fe0d7fdf44afb08a2b96c222d4c80be1157f7694e355bacd4f6d3.
