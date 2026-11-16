# Runtime Real-Spend Price Comparison

The model price page previously displayed `payable.model_multiplier` directly.
That field is a settlement multiplier, not necessarily a discount against an
official USD tariff. In particular, GPT fast entries have native CNY references
derived from a CNY 1/USD-credit supplier conversion. Comparing their `0.1` to
USD-based entries' `0.04` was misleading.

Update after authenticated Heju bills and official model-page review: prefer
the server's explicit official comparison reference. Fast must use official
Fast rates (2x Standard), not the base model's Standard rate. If no explicit
Fast reference is available, omit the ratio instead of guessing. The original
base-tier figures below are historical diagnostic evidence, not final ratios.

The Runtime now attaches a separate `comparison_reference` from a dated,
embedded models.dev provider tariff snapshot, converted by the catalog's
`fx_cny_per_usd`. Exact model IDs are used, with GPT `-fast` service aliases
mapped to the base model's public standard tariff, not an invented official
priority tariff. The UI labels the source as public reference, not verified
official pricing. Unmatched USD catalog references remain explicitly labeled
historical platform references; unmatched CNY credit references are not used.

The displayed ratio is actual payable CNY divided by reference CNY. Input and
output ratios must agree to show a single ratio; otherwise both are shown.
Long-context ratios compare matching thresholds. Cache tariffs are not implied
by the input/output ratio. Missing or invalid data never falls back to the
settlement multiplier. All payable prices and billing records are unchanged.

Observed standard-tier output examples (CNY per million, catalog FX 6.74):

| Model | Payable | Public Reference CNY | Ratio |
| --- | ---: | ---: | ---: |
| GPT-6 Astra | 13.48 | 337 | 0.04 |
| GPT-6 Astra fast | 10 | 674 (Fast) | 0.01483680 |
| GPT-6 Sol | 4.044 | 67.4 | 0.06 |
| GPT-6 Sol fast | 2.4 | 134.8 (Fast) | 0.01780415 (output) |
| GPT-6.1 Sol | 3.37 | 67.4 | 0.05 |
| GPT-6.1 Sol fast | 2 | 134.8 (Fast) | 0.01483680 |

Price digits are 16px with tabular figures. Search, sorting, and refresh share
a toolbar. The context threshold appears once per model instead of four times.
Reference comparisons expand inline and retain native keyboard behavior.

Verification: Go tests, race tests, vet, 141 Node tests, and the Playwright
billing workflow. Real public catalog screenshots cover light/dark themes at
320, 390, 600, 920, 1280, 1440, and 1920px, including overflow assertions,
readable price digits, sorting/search refresh persistence, and fast ratios.
Evidence: `/tmp/pricing-real-spend-review/.build/`.

This change is a Runtime source revision only. It does not publish a desktop
release, modify the Router/Console service, or change the production price book.
