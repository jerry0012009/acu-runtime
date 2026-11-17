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
All six Windows/Mac CLI and Windows GUI binaries were also downloaded locally
and their complete SHA256 values matched the official manifest. Upstream
Release workflow 44032290218 succeeded; Test workflow 44031852243 for the same
commit was cancelled, not a passing whole-upstream-suite qualification.

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

## Publication

Runtime v0.1.112 published at 2026-11-17T07:11:11Z from application revision
`bab093e`. Workflow `44034978995` succeeded for Windows x64, Intel Mac, Apple
Silicon and publication, without retries or removed gates. All three packaged
CLI compatibility runs explicitly reported kernel 0.1.4350, four Agent writers,
four MCP lists and preservation checks. Windows quiet Codex test passed in
53.09 seconds; installation from public 0.1.111 reached ready=true as 0.1.112,
checked actual serving kernel 0.1.4350 and unchanged config/auth hashes.

The public `/api/acu-runtime/release` returns 0.1.112 with three platform mirrors.
All five installer/DMG/ZIP artifacts are checked against published SHA256SUMS.
Linux and platform logs/evidence are under `/tmp/acu-runtime-kernel-4350/`.
This makes the update available; it does not prove every remote user installed
it. Users retain explicit confirmation and platform-specific installation steps.
No Router, Console, protocol bridge or persistent service was restarted.
