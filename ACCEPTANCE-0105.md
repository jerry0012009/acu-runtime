# Runtime 0.1.105 Price Sorting

Price request completion and application rerenders previously replaced the
entire prices page, destroying the open native select. The browser regression
failed against the previous implementation on select node identity. Price
renders now retain the toolbar and update rows/status in place. Identical rows
are not replaced, preserving expanded benchmark details. Search and selection
retain focus. Default sort is descending public capability score, with unscored
models last; manual name/price choices remain until changed in the same window.

Browser regression clicks the native select during a delayed refresh, checks
node identity/focus after completion and an application render, and checks all
four sort orders. Search, stale prices, retry, account/local tabs, logout and
responsive light/dark checks pass. Production price rows plus the same embedded
benchmark snapshot pass, with Fable5.2 first, at 1440/920/390 widths.
Screenshots: /tmp/acu-price-sort-live/.build/billing-public-prices-*.png.
137 Node tests, Go tests, race tests and ordinary Go vet pass. Linux desktop-tag
vet is unavailable without GTK4/WebKitGTK; native platform CI runs that check.

Application revision 6784c51, tag v0.1.105. Unpublished v0.1.104 build was
cancelled to incorporate the user's default-sort request in one update.
The later main test-only commit 8b4bb96 attaches the benchmark snapshot to the
optional raw public-price fixture; application assets are unchanged.

GitHub Actions run 43855785666 passed all native build/acceptance jobs and
published at 2026-11-16T11:26:31Z. Windows Setup, both macOS DMGs/app ZIPs and
SHA256SUMS are present. Public acu.best/api/acu-runtime/release returns0.1.105
with all three matching platform URLs. No backend service restart required.
