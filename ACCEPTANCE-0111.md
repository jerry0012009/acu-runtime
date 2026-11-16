# Runtime 0.1.111 Consistent Price Factors

Multiplier labels always have three fractional digits. Monetary arithmetic and
token rates are not rounded to that label precision. Runtime reads the same
published reference as pricing/monitor instead of overriding it with a local
models.dev snapshot. Supplier CNY-credit baselines are not public comparisons.

Local verification: Go tests/race/vet; 142 Node tests; billing browser fixture
with the production public catalog across desktop/mobile and both themes.
Price controls, stale-cache recovery, account isolation and history remain intact.

Router/Console reconciliation verified 3,330 acknowledged historical records
across eight price versions, including exact charges, cache modes, final quota
and zero-charge unbillable records. All 98 current payable model rates matched
the active book. No price-book changes or historic-charge rewrites are included.
Cloud fixes separately split supplier cash-cost factors from user retail factors.

Publication evidence is recorded after the three-platform workflow completes.
