# Runtime 0.1.111 Consistent Price Factors

Multiplier labels always have three fractional digits. Monetary arithmetic and
token rates are not rounded to that label precision. Runtime reads the same
published reference as pricing/monitor instead of overriding it with a local
models.dev snapshot. Supplier CNY-credit baselines are not public comparisons.

Local verification: Go tests/race/vet; 142 Node tests; billing browser fixture
with the production public catalog across desktop/mobile and both themes.
Price controls, stale-cache recovery, account isolation and history remain intact.

Router/Console reconciliation verified 3,540 acknowledged historical records
across eight price versions, including exact charges, cache modes, final quota
and zero-charge unbillable records, plus each user consumption-log amount and
quota. All 98 current payable model rates matched
the active book. No price-book changes or historic-charge rewrites are included.
Cloud fixes separately split supplier cash-cost factors from user retail factors.

Release v0.1.111 published at 2026-11-16T18:39:01Z from application revision
`5dc99a6`. Workflow `43928201434` passed Windows x64, Intel Mac, Apple Silicon
and publication without skipped release gates. Public release resolver returns
0.1.111 with three platform mirrors. Cloud Router `6e4e084` and Console/worker
`25f799b40` releases both completed with terminal-stream/unique-settlement
acceptance and revoked maintenance sessions. The production Console monitor
preserved both factors and matched public pricing across 450 comparable
Profiles. No price-book or historical charge was modified.
