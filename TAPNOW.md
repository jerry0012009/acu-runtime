# TapNow ZIP Agent Candidate

## Pending One-Click Configuration Fix

The primary command now calls `/runtime/tapnow/configure`: every attempt fetches
the latest catalog, verifies/prepares the chosen plugin ZIP, and replaces Companion's
local API credential, endpoint and model with the selected ACU provider's values.
Existing team rules/image choices and backup data are retained. Backup receives no
credential. An unchanged prepared version opens its connection page directly;
a new version opens Chrome's extension page for load/reload confirmation.
The grant requires the exact prepared version before disclosing a Key or accepting
success. Old running code cannot claim a current-version configuration. Failed
new configuration clears the old in-memory success indicator. The guide's Continue
action only resumes an already prepared version; it does not repeat the download.
This fix is not included in the already published 0.1.80 installer until released
as a subsequent version. Old manually loaded extensions with a different ID are
not overwritten; load the managed directory and explicitly disable the old copy.

Status: Runtime 0.1.80 release preparation, with user-authorized first public
distribution for device testing. Publication is not complete until the GitHub
release assets exist and all required CI jobs pass. ZIP-based
unpacked installation is the selected distribution method. Chrome Web Store
registration, publisher access and review are **not** prerequisites for this flow.

## User Flow

Other Agents and Install Other Agents are collapsed by default. TapNow is optional
and excluded from native Agent batch setup. Backup and Companion are independent;
Backup never needs or receives an ACU Key.

Runtime downloads the chosen ZIP from `jerry0012009/acu-tapnow`, checks exact URL,
SHA-256 and size, safely extracts it into a fixed current-user directory, checks
its fixed extension identity, registers its current-user Native Messaging host,
and opens the directory plus `chrome://extensions/` in Google Chrome.

First loading still requires Chrome's developer-mode switch and Load Unpacked
directory confirmation. Runtime does not claim to bypass this, edit protected
Chrome preferences, inject enterprise policy or stop the user's browser. ZIPs are
extracted first, not treated as store-signed CRX files or dragged ZIP installs.
An already installed copy can use Connect without another download.

On first installation the extension opens its connection page automatically.
The five-minute Runtime grant carries the selected provider/model. Companion
receives Key, exact ACU endpoint and protocol over Chrome Native Messaging, saves
the configuration locally, reads back individual fields, then acknowledges.
No credential download, copy/paste or manual JSON import is needed in this flow.
Rules, prompt and image choices remain unchanged. Status says configuration
confirmed, with version/time, not live browser presence or successful inference.

Supported ACU endpoints: `https://api.acucompute.com/v1`, `https://acu.best/v1`.
Connection does not call a paid model. TapNow login and user backup-directory
permissions remain browser-managed. Existing manually installed copies are not
silently removed or claimed. Disable an old copy before loading the new one to
avoid duplicate content-script UI; changing the old extension ID does not migrate
its local storage automatically.

## Fixed Identities And Updates

Both ZIP manifests contain a stable public `key`; identity derivation is checked
at build time. Companion: `illffggofpdcjmdcenajnbmnokoclabh`.
Backup: `lohhkeighnafehcjelhlimegfgpelghg`.
Keep these keys across independent plugin updates so moving the unpacked directory
does not create a new extension ID. These public keys pin IDs, **not code signing**:
unsigned unpacked code can copy them. Trust the verified publisher ZIP and local
OS account; Native Messaging IDs alone do not authenticate package bytes.

The schema-1 GitHub catalog lists separate versions, fixed release URLs, hashes
and sizes. Install fetches this catalog and prepares only the chosen edition.
Update stages and swaps the same directory, retaining `.previous`; failed
verification leaves current files unchanged. User clicks Reload in Chrome, then
Connect if necessary. Browser storage and backup data are outside Runtime's code
directory. Process termination during a swap may need manual `.previous` recovery.
Do not promise Chrome auto-updates unpacked extensions.

`acu-tapnow` source export excludes old Git history, real canvas examples, old ZIPs
and trial Key files. Original TapNow baseline: `f3df6c6`. Review future upstream
changes, test each ZIP, publish assets before activating their catalog. Never
overwrite released bytes. Compatible plugin updates do not require Runtime
rebuild; identity, permission or protocol changes need coordinated acceptance.

## Native Channel

Host `com.acucompute.tapnow` runs the Runtime executable's early stdio-only branch.
Host registration accepts only the two fixed extension origins. Windows uses
current-user registry; macOS default Chrome user-data directory. Custom Chrome
data directories are not automatically discovered or changed.

A private rendezvous file contains only loopback origin, native token and expiry,
never an API Key. Native endpoint rejects browser Origin/Fetch-Site and ordinary
Runtime session tokens. Claims bind edition, origin, provider, model and expiry;
one-use claim plus same-grant readback receipt prevents accidental repeated writes.
Retries, multiple connect pages, JSON key reordering and profile mismatch are
covered. Key remains in extension local storage, not Chrome sync, URLs or logs.
Pending cancellation does not revoke an already delivered Key; revoke it in ACU.

## Verification

Go tests/race checks, 125 frontend tests, plugin tests, both ZIP builds and real
MV3/native host roundtrip are exercised locally. The native smoke test loads the
actual built manifest keys without injecting new keys, then verifies configuration
and receipt without cloud/inference calls:

```sh
ACU_PLAYWRIGHT=/path/to/node_modules/playwright \
TAPNOW_TEST_ROOT=/path/to/tapnow/.output \
node scripts/test-tapnow-native.cjs
```

This is Chromium's isolated load-extension harness, **not** proof that ordinary
Google Chrome's developer-mode UI or Windows/macOS registration was exercised.
Release CI covers Windows/macOS native builds and isolated extension-to-host
configuration. User device testing after publication covers actual ZIP Load
Unpacked, same-directory update/reload with retained settings, Runtime restart,
Chrome disabled/blocked behavior and authorized live-canvas compatibility. These
remain disclosed acceptance gaps, not prerequisites for the user to download the
explicitly requested first public version. Existing content/LLM TypeScript diagnostics remain.
Linux native desktop build here lacks GTK4/WebKitGTK6 development libraries.

Reference: https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging

## Guided Setup Closeout (2026-11-15)

The agreed experience is assisted first installation, not a claim of fully
automatic browser setup. The Runtime shows file preparation progress, a three-step
Chrome loading guide, a copy-directory control, reopen-browser/directory controls,
and cancellation. These remain available while waiting for the extension receipt.
Canceling preparation interrupts its context, keeps completed files, and does not
leave a grant eligible for subsequent claim. Canceling an already claimed grant
cannot retract bytes previously delivered; revocation remains an ACU Key action.
Retained files can continue setup without another download. A plugin update check
is explicit, with separate per-edition update actions. No new inference is sent.

The connection page includes an Open TapNow link after successful local save and
receipt. Existing TapNow tabs may need one refresh. The help section covers old
duplicate extensions and expired grants. Runtime reports file readiness and
configuration confirmation separately, without treating directory/browser opening
as successful installation.

Verification: 125 Runtime frontend unit tests; Go race tests and vet; 85 plugin
tests; both ZIP builds with identity/entry checks; real native-message roundtrip;
8 guide viewport/theme screenshots. Browser coverage exercises waiting, reopening
tools, cancellation, continuing without reinstall, and receipt-driven completion.
No Windows/macOS native or authorized real-canvas acceptance is inferred from
these isolated Linux/Chromium checks. No public release/tag/channel activation was
performed for this closeout.

## One-Click Investigation (2026-11-15)

Clicking the enable command is the authorization for installation preparation and
selected Key configuration; there is no extra consent checkbox. Model/Key storage
and potential inference charges are disclosed beside the command.

Ordinary Google Chrome 157.0.8103.78 was tested with isolated profiles, not just
Playwright Chromium. `--load-extension` did not load the extension. The browser
CDP `Extensions.loadUnpacked` did load it and the native receipt completed, but
closing that installer and reopening Chrome normally produced
`ERR_BLOCKED_BY_CLIENT` for the extension page. Asking Chrome's own extensions UI
to enable developer mode in that dedicated profile did not resolve the restart
failure. Thus the experiment did not pass durable installation acceptance.

The dedicated automatic installer was removed, not offered to users or silently
substituted for their current browser. No default-profile files, policies, Chrome
security controls or existing browser processes were modified. The non-mutating
isolated probe remains at `scripts/probe-chrome-extension-install.cjs`; local
result: `.build/chrome-install-probe.json`. We do not infer that all possible
platform automation is impossible, only that the tested path is not accepted.
