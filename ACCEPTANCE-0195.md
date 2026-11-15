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

The tag release pipeline must pass Windows x64 and both macOS jobs, including
native desktop checks and packaging, before publishing the complete release.
Local Linux checks do not substitute for those native checks.
