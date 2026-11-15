# Runtime 0.1.88 Guided Setup

User successfully installed the previous version through their own understanding,
not the product guidance. Requested actionable instructions and maximum application
assistance instead of default technical explanation or defensive warnings.

## Changes

The default page removes the cross-edition popup-warning block, ERR_BLOCKED_BY_CLIENT,
organization-policy text, redundant backend messages and long consent copy. Existing
backend identity/expiry/Key protections remain unchanged. Short charge notice remains
beside the model choice. Actual failures remain errors, never converted to success.

Each edition has a sequential guide: open extensions page, load directory, connect.
First-time configuration copies the extensions address and launches Chrome. Proceeding
to loading copies the directory and opens its folder. Exact Chrome controls and
positions are named. Directory confirmation refreshes the Runtime grant and opens
TapNow. Same prepared version goes directly to connection, not developer-mode steps.
The user's Next command is a navigation choice, not installation proof. Only the
existing native-message readback receipt ends the flow. Success provides Open TapNow.

Clipboard first uses the browser API, then a session-protected Windows/macOS native
fallback, finally a selectable field. Native endpoints accept only the fixed
extensions address or a prepared edition's managed path, not arbitrary text/paths.
PowerShell receives UTF-8 stdin; macOS uses pbcopy. Disposable release runners test
roundtrip with Chinese/spaced paths and preserve their prior text where available.
Normal unit tests never modify a real clipboard. Backend URLs/Key configuration and
browser permissions are not weakened or force-modified. No system keystroke injection
or direct Chrome protected-preference edit is introduced.

Plugin 0.1.26 shortens first-install and action-popup status to connecting, connected
or the next necessary Runtime action. No key/protocol change or identity migration.

## Verification

Local Go race/vet, 132 frontend tests, plugin 88 tests and both builds. Browser tests
cover automatic first-step address copy, next-step directory copy/open, all three
steps, cancellation/continuation, successful completion/Open TapNow, same-version
connection shortcut, native clipboard failure fallback and no default technical
warning text. Eight widths/themes per step are captured. Real MV3/native fixture
still verifies exact Key/model save and preserved rules; Backup receives no Key.

Release CI retains Windows x64 and macOS arm64/amd64 native build/upgrade gates,
adds clipboard roundtrip smoke, and pins the matching released plugin ZIPs. Owner
Chrome native dialogs and toolbar are still owner-device acceptance, not inferred
from fixture Next clicks. User already reported prior installation success; no claim
that this host's CLI can operate the remote owner's PC is made.

Plugin source `482dbaa1cba739b05ae47780f48b3392415297f0`, workflow 43710051590
passed tests/audit/build. Published 0.1.26 ZIPs were downloaded and rehashed:
Companion 45,259 bytes, `6d5799dd7c09c1dfae4861d6ea3b6aea68c932316bf2de91f04e150a2dc617dc`;
Backup 189,804 bytes, `624f9edbe96b15439f2ba88a3d5cb82794d1eb8517ebd9009ab82f1094487833`.
