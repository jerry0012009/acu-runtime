# Runtime 0.1.95: Complete Model Prices

## Scope

Runtime's model-price page now defaults to ascending published output price.
It displays the published multiplier and all four payable prices: output,
input, cache hit and cache write. Prices already include the multiplier.
Every available long-context price appears with its token threshold.
Missing/invalid prices stay distinct from zero; small cache prices retain
significant digits. ACU Auto remains dynamically billed, without fixed prices.

The desktop table highlights output prices; narrow windows use labeled price
grids. No backend, accounting, model price, kernel or production service changes.

## Local Verification

- `go test ./...` and `go vet ./...` passed on Linux.
- `node --test tests/*.test.cjs`: 132 passed.
- `node --check web/billing.js` passed.
- `scripts/test-billing.cjs`: the updated full-price assertion failed on the
  old implementation, then passed after the implementation change.
- Playwright checks complete prices, multipliers, zero/missing/invalid values,
  precision, all four context tiers, ascending/descending/name ordering,
  search, refresh, cached values, failure recovery and account isolation.
- Light/dark screenshots at widths 1440, 1280, 920, 600 and 390; no page overflow,
  no overlapping adjacent price columns, and long model identifiers wrap.
- Additional screenshots use the unauthenticated official public price catalog.
- Evidence: `/tmp/acu-runtime-price-check/.build/billing-*.png` on this host.

## Publication

- Published at `2026-11-15T15:49:30Z`, tag `v0.1.95`, application revision
  `f107bafed3771ac31a6c8447d06544547a352bff`.
- [Release](https://github.com/jerry0012009/acu-runtime/releases/tag/v0.1.95)
  is public, not draft or prerelease; the latest-release API returned `v0.1.95`.
- [Workflow 43729903841](https://github.com/jerry0012009/acu-runtime/actions/runs/43729903841)
  completed successfully: Windows x64, macOS ARM64, macOS AMD64 and publication.
- Attempt 1: Windows' third native-login scenario timed out waiting for
  `#quick-key`; both earlier native-login/restoration cases and price-page
  checks passed. After inspecting the failure, reran only failed jobs with
  unchanged source. Attempt 2 passed all Windows native, install, upgrade and
  kernel compatibility checks. No checks bypassed; Mac artifacts retained.
- Five installers/app archives, `INSTALL.md` and `SHA256SUMS.txt` are present.
  Windows installer SHA-256:
  `5402b1746e06e00d2b3e3c2f27092f4bbc58a8db4afa316906bf7d65695eedc1`.
- Main advanced concurrently with later onboarding work; the published price
  release remains the immutable revision above. No production API release,
  service restart, model invocation or billing mutation was performed.
