# Video Reading And TapNow Connection Recovery

Video reading is default-on for connected desktop Agents and is prepared once
per selected-provider/Agent set on window startup. Explicit off is preserved in
the existing official Library paused setting. The independent reading toggle
and repair action do not configure unrelated generation/search tools. Readiness
and enabled preference are separate; a checked default does not claim a missing
tool has already been installed. No media decoding or Magpie patch was added.

TapNow configured grants remain confirmable after the unclaimed grant timeout.
Pending local acknowledgements can resume with the same grant; a new explicit
Runtime authorization supersedes a stale local receipt. Native origin/token and
receipt/version checks remain enforced. Popup errors name the affected edition
and recovery command. Already valid local settings remain usable without
claiming the Runtime handshake succeeded or overwriting credentials.

Backup is optional and its inactive guide stays collapsed independently of the
Companion completion state. Companion's completed section always offers reconnect.

Verification: Go tests/race/vet, 133 frontend behavior tests, 96 extension tests,
both extension ZIP builds, zero added TypeScript diagnostics against main.
Playwright checked independent reading toggle/repair and disabled state across
390/1440 widths and light/dark; TapNow setup checked 320/390/768/1440 in both
themes with screenshots and no overflow. Actual MV3 -> Chrome native host ->
authenticated Runtime -> local readback -> acknowledgement test passed for both
editions, with no key in Backup and zero cloud calls.

Linux native desktop-tag compilation is unavailable on this host because GTK4 /
WebKit development packages are absent. Native Windows/macOS verification is
performed by the desktop release workflow; end-user private Chrome profile and
organization-policy behavior are not inferred from the isolated fixture.
