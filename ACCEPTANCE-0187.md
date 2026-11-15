# Runtime 0.1.87 Plugin Popup Connection

User reported the local Companion ID `illffggofpdcjmdcenajnbmnokoclabh` shows
ERR_BLOCKED_BY_CLIENT. Runtime still launched chrome-extension://.../connect.html
from an external process on reconnection after the 0.1.86 extensions-page fix.
Such navigation is not reliable and can also fail when the extension is absent,
disabled, loaded in another profile or blocked by organization policy. No owner
browser state was inspected, so no one specific Chrome condition is asserted.

Runtime no longer launches any extension page. Reconnection opens the normal
TapNow website and instructs users to open the installed extension using Chrome's
extensions menu. Launch-argument normalization also replaces accidentally passed
chrome-extension targets with the normal website. Browser launch is not a proof
of plugin presence or installation.

Plugin 0.1.25 provides an identical native connection command in Companion and
Backup popups. Opening a popup attempts a connection only to an existing Runtime
grant; no grant causes no credential write. Button retry and concurrent opening
are guarded by the existing per-extension browser lock. Companion reads back the
Key/model before acknowledgment and refreshes the displayed settings. Backup
receives no Key. No external messaging, web-accessible resources, force-install
policy or browser profile modification is added. The connect page remains an
extension-owned first-install fallback, not an externally launched URL.

Local verification: Go race/vet and launch/handler tests; plugin 88 tests including
popup no-grant/retry/double-click behavior; both identity-verified ZIP builds;
real MV3 popup -> native host -> Runtime -> local credential readback/receipt for
both editions. The smoke fixture closes first-install pages before seeding the
old Key to avoid an artificial install-page claim racing with the popup test.
The test opens popup.html inside the isolated browser context; it is not evidence
of a successful external plugin-page launch or a user's browser toolbar click.

Chrome's ERR_BLOCKED_BY_CLIENT is not treated as evidence of a remote installation
or missing local bytes. User guidance checks Load Unpacked, enabled state, load
errors, correct Chrome profile and organization policy; it does not suggest
turning off browser security or making private extension pages web-accessible.

Plugin source `af505e4c0b6b26ab4cb2ac6cab1da37c73955375`, workflow 43706347667
passed tests/audit/build. Published 0.1.25 ZIPs were downloaded and rehashed:
Companion 45,361 bytes, `6fdaf136fd757dc6a6016cb9b65f7b281d986f80349c8e564fc6ab2074008783`;
Backup 189,901 bytes, `e1e39cd3f4e92af4ff3fecb9062f9bd0efd6f22d731653a7f27ad70059ed3cee`.
Runtime release CI pins those exact hashes for all three native platform jobs.
