# Runtime 0.1.100 Account Billing

Scope is only Usage and Fees / ACU Account. Workbench home, local kernel usage,
model-price page, model selection, official Magpie kernel and charge rules remain
unchanged. The only app.js change passes the existing icon helper to billing.

The selected-Key status block is removed, and its cloud detail request no longer
runs for account billing. Account totals are rendered in an unframed three-column
band, with two-decimal overview amounts and unchanged precise log amounts.
Comparison expands on demand. The account menu contains logout. Whole-range
canonical model ranking supports cost/count order, five/all rows and responsive
light/dark layout. New stylesheet selectors are scoped to the account area.

Cloud aggregation matches historical reference amounts to their actual charges;
official, platform and OpenRouter labels remain distinct. No present-day tariff
recalculation, forced positive savings or auto-alias model attribution is used.
Missing comparison data leaves an unavailable state; opposite premium and empty
states are exercised. Date selection controls cost/comparison/ranking together.

Verification:
- Local Go race suite and vet passed; 134 JavaScript unit tests passed.
- Playwright billing regression passed, including unchanged local iframe/prices,
  pagination, failure retention, account isolation, comparison states, sorting,
  expand/collapse and light/dark widths 390/600/920/1440.
- Real account-122 cloud overview reconciled all model totals with account total;
  account was rendered in both themes at 390/1440. No Key status block present.
  Maintenance session revoked with HTTP 401 reuse verification.
- Server release `bg-20261116T050323-952d85ab` completed, app `1e7eeb71e`, with
  public/candidate protocol terminal events and unique acknowledged settlement.
- Runtime app `a68bca1`, tag v0.1.100, Actions run `43807114633`: Windows x64,
  macOS Intel and Apple Silicon builds and publish all succeeded, including
  native desktop, clean installation, old-version upgrade and kernel compatibility.
- Published 2026-11-16T05:08:38Z. Official website release resolver returns 0.1.100.
- Downloaded Windows installer and both app ZIPs match published SHA256SUMS:
  Windows `4b27d46753f44925bfd50060fafecdedea1b0fd157adf3c12dcb608430cccb49`;
  Intel ZIP `ecc693a5816cb604a41995e8e4faf9268131ac397ea885d3477e4192638df1e2`;
  ARM ZIP `f91a3a3acc1faf0187ce35f04d7dcfcd5abcc89044ddecbefdc06a78c17d3537`.

Private screenshots/reports: `/tmp/acu-account-billing-visual/.build` and
`/tmp/acu-account-overview-live`. Real-data UI test uses a browser shell, not
control of the owner's native PC. An isolated developer server was stopped after
verification. Source and remote main include the release; later acceptance docs
do not require rebuilding the already verified installers.
