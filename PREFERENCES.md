# 我的偏好

Runtime adds a read-only personal preference page without requiring an account
for the existing model Key workflow. Preference-only native login does not
select, reveal or apply a model Key. Existing official WebView authentication
and background access-token renewal remain in use.

The contract is Console main `cdf8172`,
`docs/implementation/2026-11-12-runtime-preferences.md`. Console application
`b3a309d58` was deployed on 2026-11-12. No Console or Router release is required
by this Runtime change.

The local authenticated `/runtime/preferences/{profile,library,activity,settings}`
GET routes read the corresponding official account endpoints. Library and
activity require version 1. Cloud access tokens remain in Go memory. An opaque
per-account/session identity scopes the memory-only 60-second page cache.
Logout, account switch and confirmed authorization failure clear private data;
in-flight replies from a previous session cannot populate the new page.

The page shows saved interpretations, full source descriptions, domains and
scope, plus recent summary changes with collapsed previous summaries. It does
not display source/run IDs, raw conversations, Markdown diffs, accuracy or
growth counters. Identical updated summaries are omitted. Empty, partial,
unsupported, failed and saved/stale profile states remain distinct. Learning
and reference settings are read only; opening the page never changes them.

## Verification

```powershell
go test ./...
go test -tags desktop,production ./...
go vet -tags desktop,production ./...
node --test tests/*.test.cjs
node scripts/test-preferences.cjs
go test -c -tags desktop,production -o ../.build/preferences-native-smoke.exe
$env:ACU_NATIVE_TEST_EXE = (Resolve-Path ../.build/preferences-native-smoke.exe).Path
node scripts/test-native-login.cjs
```

Browser tests use an isolated loopback fixture, not a production account. They
check 320/390/768/1440px in both themes, long Chinese/English text, escaping,
search, keyboard comparisons, missing sources, unchanged summaries, saved
profiles, empty/404/error/revoked responses, cache, late account-switch replies
and logout. Tests never send paid model requests or alter real Agent files.

Windows native tests use an isolated profile and instance ID, covering login,
Key switching, access expiry, transient renewal failure and recovery, plus the
preference page and keyboard comparison. They do not close the user's Runtime.
Screenshots are written to `output/runtime-preferences`.

For CI or nonstandard layouts, set `PLAYWRIGHT_MODULE` to the installed
Playwright directory and `ACU_TEST_WORKSPACE` to an artifact workspace. Optional
`CHROMIUM_PATH` selects the test browser. macOS CI builds and starts the actual
native app; a fixture browser run is distinct from authenticated native
production acceptance.
