# Runtime GPT-6.2 Price List Benchmarks

The operator's model-price list already contains GPT-6.2 Sol and Astra, including
correct payable rates, multipliers and >272000 input-token context tiers. The
missing part is their capability reference and capability sorting: Runtime's
embedded AA snapshot was retrieved before GPT-6.2 was released. Refreshing public
prices cannot update a benchmark snapshot compiled into an older application.

Runtime 0.1.122 adds only the two exact release entries, retaining every previous
model's benchmark variants. Source: Artificial Analysis Intelligence Index
v5.0.1 public leaderboard, retrieved 2026-11-18; SHA256
`8b718fb5c869a261e29293675f8452bda822da52dd4e8f9408c6e5319baf6632`.
Per-model observation dates avoid implying that older entries were refreshed.

| Model | Highest Public Score | Display | Variants |
| --- | ---: | ---: | ---: |
| gpt-6.2-astra | 62.1387990593969 | 62.1 | 5 |
| gpt-6.2-sol | 57.2576209885477 | 57.3 | 5 |

The existing highest-published-configuration sort is preserved, not replaced
with the cloud Auto medium-effort curve. Unmeasured fast/preview aliases never
inherit scores. Price amounts and cloud routing are unchanged. New benchmark
metadata is attached even to cached public prices after the Runtime upgrade.

Verification includes a pre-fix failing regression for missing GPT-6.2 scores,
passing Go test/vet and 167 Node tests, exact price/benchmark attachment and
cached-price enrichment. A read-only production price fetch returned 134 models
and 23 exact benchmark matches. No inference or live user configuration writes.

Browser verification uses the enriched real production catalog, not a hand-built
price list. It checks both new models in capability order above predecessors,
all four standard/long rates, exact multipliers, expanded effort references,
search, refresh retention, and 390/1440 widths in light/dark. Existing full
billing behavior and responsive checks remain enabled. Private evidence and
screenshots: `/root/jerry/exports/runtime-gpt62-pricing-20261118`.

Native Windows and both macOS installers are published through the existing
three-platform release workflow after its native validation and packaging gates.
No Router, Console or Private application restart is required for this change.
