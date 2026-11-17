# Runtime 0.1.112 / Magpie 0.1.4350

This release updates the user-side bundled official CLI from 0.1.4248 to
0.1.4350 (upstream release commit 618bfc1a95879c3fb780fb01a94868a523a22e86).
It is separate from the cloud protocol-bridge release. No Magpie source patch,
cloud application restart, customer billing change or credential migration is
part of this desktop update. Version 0.1.4350 was the latest official stable
release when selected on November 17, not a continuously moving latest target.

All four CLI platform/architecture hashes and both Windows GUI upgrade hashes
come from the official 0.1.4350 SHA256SUMS. Linux qualification used the official
CLI, verified SHA256 a0fe7a75b78eb584a2547e02dd45fc9d1b37b229085b68758b6aa4c9cadc1a27.

Local isolated official-binary compatibility passed: four Agent configuration
writers, native update-policy commands, existing catalog refresh, explicit and
automatic shared models, native video default/explicit override, four MCP tool
lists, repeated assignments without duplication, original MCP preservation,
model-only saves not rewriting Agent files, and fresh Codex API configuration
without manufacturing a ChatGPT login. Separate official-kernel native catalog
sync passed. Full Go tests/race/vet and 142 Node regressions passed before
release preparation; added idle-upgrade boundary tests reject unknown/busy
request, waiting and tool counts.

Release CI tests Windows/macOS native desktop builds and actual packaged CLI.
Windows upgrade acceptance uses current public Runtime 0.1.111, verifies the
new serving kernel version and unchanged Codex configuration/authentication
hashes. Existing quiet Codex rollback, cancellation and disconnect checks remain.
This is not a claim that every remote user's device has already upgraded.

Windows installs the bundled CLI and checks/backs up only the verified default
independent GUI path after idle. Custom paths remain untouched and newer GUI
binaries are not downgraded. macOS users replace Runtime with the new DMG;
an old separately running Magpie must be updated through its own updater or
closed before reconnecting. No unattended force-stop is added on macOS.

Three-platform publication evidence will be added after CI completion.
