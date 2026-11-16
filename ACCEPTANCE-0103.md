# Runtime 0.1.103 Account Loading Recovery

The account view runs overview, balance, currency and log reads concurrently.
Successful overview responses supply the exact/historical summary, avoiding
another full log-stat aggregation. Fallback remains available if overview fails.
Account identity changes and private log-metadata filtering retain their guards.
The main workbench is unchanged by this account-loading patch.

The corresponding Console fix is application revision
`e3aafc738c52d506a5021ab1f2c2acc821891f40`, released with manifest
`bg-20261116T105708-e3e9c1e3`, status `completed`. It preserves one comparable
reference cohort when another source appears, streams rows and extracts only
billing fields with the existing GJSON library, and uses a 30-second
account/range-scoped cache with coalesced concurrent reads.
Historical platform references are not relabeled as official tariffs.

Production acceptance on 2026-11-16 used account `wzr0012026` through the public
API and then the actual Runtime `accountBilling` handler, not only a UI fixture:

- Before: overview 12,494 ms; comparison unavailable; redundant stat 1,767 ms.
- The first SQL optimization reached 4,726 / 117 ms cold/warm for 7 days,
  but the 30-day overview still exceeded 20 seconds. It was superseded by
  streamed extraction after expanding acceptance to all available ranges.
- Final complete Runtime handler: 7 days 981 / 183 ms cold/warm;
  30 days 3,394 / 190 ms, with 8,650 records and no warnings.
  Network and load vary; these are host-side measurements,
  not owner-PC timing or a subsecond cold-load guarantee.
- 1,772 consumption records, total CNY 25.8293730087, model totals reconcile.
- Comparable platform cohort: 1,050 records, reference CNY 260.7514500424,
  ACU charge CNY 21.0438298408, difference CNY 239.7076202016 (91.93%).
- No warnings. Summary and overview exact/historical counts reconcile.
- Light/dark browser shells at 390/1440 pixels render the actual Runtime
  handler response, percentage and expandable comparison, with no page overflow.
  The narrow consumption table retains its own horizontal scroll.
- Go race tests and targeted Console model/controller/service tests passed.
  A barrier test requires the four cloud reads to start together and prohibits
  an unnecessary stat call on successful overview.
- Candidate/public release acceptance each completed three real protocol
  streams and unique finalized usage. HTTP and singleton worker are healthy
  on the selected revision with `unless-stopped`; old HTTP slot retired.
- All release/measurement sessions were revoked and reuse returned HTTP 401;
  private session token files removed.

Opt-in live regression, using an explicitly authorized temporary session:

```bash
ACU_BILLING_LIVE_SESSION=/private/session.json \
ACU_BILLING_LIVE_OUTPUT=/private/runtime-result.json \
go test -run '^TestAccountBillingLiveAcceptance$' -v .
```

Set `ACU_BILLING_LIVE_DAYS=1`, `7` (default) or `30` for each supported range.
The session owner must revoke it after testing; the test does not log tokens.
Raw evidence and screenshots are retained privately under
`/tmp/acu-billing-recovery/` and are not committed with account data.

Release `v0.1.103` published 2026-11-16T10:48:23Z at source
`5886989736597944082e18eef8d7b33f61ed86a0`. Workflow `43849055385` passed all
three native builds and publication. Its ARM runner initially could not be
allocated; retry succeeded without changing application source. Public release
resolver returns 0.1.103 and all three mirrors. Downloaded Windows installer
SHA256 matches published `e36bbc5be11b3c8e6818ee9ec72d1873e2b7a3a398bf232281ada62f05c5a773`.
Later main changes are tests/documentation only, not a rebuilt application.
