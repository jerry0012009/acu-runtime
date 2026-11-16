# Runtime 0.1.108 Responsive Tool Setup

Model setup no longer waits for the full media/download job. Runtime tracks a
single bounded background task per provider, coalesces additional Agent scope,
and exposes real stages/download percentage. The existing kernel still owns
tool definitions, catalog selection, MCP assignments and client-file syncing.
The task is tied to Runtime lifetime, not a browser request. Navigation/polling
continues; failure retains model/basic-tool completion and offers tools-only
retry. UI reload can reattach to a running task. Different-provider jobs do not
silently replace the current task.

Existing tools use Magpie's `servers/agents-some` batch operation rather than
three separate assignment writes. Fresh installs/migrations retain the native
market and customization guards. Settings/library reads run concurrently.
Audio env writes are skipped when unchanged; local credential lookup no longer
refreshes the provider catalog. Explicit model permissions remain checked.

Video downloads reuse existing Runtime mirrors, redirect policy and atomic,
SHA256-checked installation. Preparation is serialized and caches successful
unchanged binary identity for ten minutes; changed files and expired cache
are revalidated. Browser native registration now belongs to actual TapNow
Companion setup, not every ordinary Agent configuration. Download does not
hold the capability mutation lock; opt-out is rechecked before final assignment.

Pre-release verification:
- Full Go race/vet and 137 Node regressions passed during implementation.
- Repeat capability fixture: 47ms, no config mutation or provider refresh;
  adding an Agent uses one native batch assignment.
- Official Magpie 0.1.4248 isolated tool install/assignment: first 1,666ms,
  repeated read/check 23ms. Actual WorkBuddy files retain new model IDs.
- Packaged video tool cold extraction 29,255ms; immediate repeat <1ms.
  Changed binary is rejected from the cached identity. These are host-side
  isolated measurements, not end-user full-setup latency promises.
- Browser acceptance covers pending download, usable navigation, failure,
  tools-only retry and completion at 390/1440, light/dark; no page overflow.
- Background-task tests hold video preparation pending while proving POST
  returns and config mutex is free; duplicate scopes coalesce and complete.
- Release workflow repeats browser and official-kernel checks on all platforms.

Screenshots: `/tmp/acu-setup-browser/output/setup-progress/` (private fixture).
