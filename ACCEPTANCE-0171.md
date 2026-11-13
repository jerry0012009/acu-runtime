# Runtime 0.1.71 Kernel Update

## Package

Latest stable Magpie at verification: v0.1.4096, published 2026-11-13 22:02:13
UTC. Official CLI binaries are unmodified. Windows x64/arm64 and macOS
Intel/Apple Silicon digests match the upstream SHA256SUMS. Linux acceptance
binary SHA-256: 2f064ecbd5539746c3e04ec28b572e12e79ece2b4c86833f6f39df9e7cf5dff8.

Changes reviewed between v0.1.3940 and v0.1.4096 include Codex provider-table
ownership, model deselection, desktop catalog refresh and native MCP local-start
options. Existing Runtime quiet transactions, login policy and ACU display-only
branding are retained; no fork or new compatibility layer added.

## Verification

- Go race suite, vet and 123 frontend tests pass.
- Official v0.1.4096 + real installed Codex: selected model/config load, ACU
  provider display name after kernel restart, all three login modes, invalid
  model rollback, cancellation, disconnect and unrelated MCP preservation pass.
- Isolated four-Agent Runtime/native integration passes Codex, Claude, Hermes,
  WorkBuddy, native model settings, legacy tool migration, repeated assignments,
  external MCP preservation and stdio tools/list for vision/image/search.
- Native v0.1.4096 UI passes concise login/brand tests, light/dark and mobile/
  desktop widths; no technical identifiers or links changed.
- Release installer upgrade fixture starts at 0.1.70 (v0.1.3940 kernel), then
  verifies new Runtime/kernel versions and unchanged Codex config/auth hashes.
- Compatibility harness explicitly checks serving kernel version against pin.
- Live native MCP calls passed GPT-6.1 Sol recognition (LEFT/RIGHT/FF9A518F,
  red/blue shapes), keyless search, example.com fetch and Sunburst generation
  saved as a 1,089,323-byte image. First automatic vision choice Grok failed
  upstream with model unavailable; selecting a native candidate in isolated
  settings passed. No per-model Runtime override added. This records a supply
  limitation, not a blanket guarantee all upstream models are callable.
- Temporary bounded acceptance Key revoked; reuse rejected with HTTP 401.

Only desktop Runtime packages change; no Router/Console/Private deployment or
production restart. Windows ARM64 digest is maintained but no Windows ARM64
Runtime installer is offered or claimed tested.

Evidence: /tmp/acu-kernel-4096; native UI screenshots under browser/output.
