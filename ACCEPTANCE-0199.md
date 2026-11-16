# Runtime 0.1.99: Public Capability Reference

Price sorting adds capability descending while retaining default output-price
ascending. Each scored model has a collapsed reference under its name, with
benchmark version, all measured configurations and scores, snapshot date and
the model's Artificial Analysis source page. Unscored entries follow scored
entries with stable name ordering. Auto, image/video and unmatched releases do
not inherit a text benchmark or get a fabricated score. Price data is unchanged.

Source: https://artificialanalysis.ai/leaderboards/models retrieved 2026-11-16.
The public page's structured model data reports Intelligence Index v5.0.1.
The committed snapshot contains 58 model releases with non-null, finite index
scores. Each release's highest published configuration is used for ordering;
details expose the actual configurations, not a claim about a user's selected
reasoning effort. Only exact release slugs match, with dot/hyphen punctuation
normalization. Fast/preview variants do not silently reuse a family score.
No old ACU fitted curves, mixed benchmark versions, family deltas or raw
SWE-bench percentages are used as a replacement index.

Runtime enriches the existing public price response at read time, including
disk-cached prices. No account/API credentials enter the reference snapshot,
no third-party fetch blocks the user's UI, and no Router/Console release or
database migration is needed. The source HTML SHA-256 and observation date
are retained with the generated snapshot. To refresh, download the public
leaderboard HTML and run:

```sh
node scripts/refresh-model-benchmarks.mjs LEADERBOARD_HTML
```

The parser validates source/version/data before replacing the snapshot. Review
the diff and run the focused tests; publish through the normal Runtime release.
The UI's date makes snapshot age visible. It is not advertised as live scoring.

Verification:
- Go tests, race tests, vet; 134 Node tests passed.
- Parser test covers multi-configuration scores, null scores and invalid input.
- Go coverage verifies exact release matching, no invented aliases, unchanged
  prices, all snapshot score ranges and descending configuration order.
- Real public-price read matched 21 of 137 current price rows, including Fable
  5.2, Opus 5.5, GPT-6.1 Sol, Astra, Gemini 4 and Kimi K3.1 Pro.
- Playwright verifies score ordering, missing/invalid scores, collapsed details,
  source links, HTML escaping and unsafe-link rejection, refresh/search, existing
  price sorting and responsiveness at 390/600/920/1280/1440 in both themes.
- Real-catalog screenshots at 390/920/1440 show Fable 5.2 first at 64.6, with
  detailed configurations and prices. Evidence: `/tmp/acu-capability-sort-live/`.

This adds a price-comparison reference only. No routing model, learning behavior,
provider configuration, tariff, Magpie version or default Agent choice changes.
