# Runtime 0.1.114 Mac In-App Updates

Adds architecture-specific official app.zip selection for Mac update checks.
Downloads reuse existing public mirrors and full SHA256 validation; application
identity, bundle version, ad-hoc signature integrity, Mach-O architecture and
packaged kernel manifest/version are verified before quitting. No claim of
Developer ID signing or notarization is made.

A detached copy of the current executable waits for the exact old PID/start
identity to exit. A same-volume staged application replaces the old bundle
with a full backup retained until the new PID reports desktop ready, expected
version and expected kernel version. Launch failure restores the old bundle.
Unknown/busy kernel state prevents handoff. Writable bundle parents only;
mounted DMGs, translocated paths and linked app roots are rejected. No sudo,
password capture, permission bypass or user configuration migration.

Settings no longer repeats Agent installation/update information. Existing
Workbench controls remain. Older Mac clients must install this release manually
once, because earlier clients cannot run the new updater themselves.

Tests cover exact Mac architecture/package matching, invalid release metadata,
unsafe archive paths/symlinks, native helper PID handoff and rollback, user config
preservation, and Settings controls. Native Mac tests run against the signed
packaged application on both Intel and Apple Silicon. Cross compilation is not
reported as native acceptance.

Candidate workflow 44040193051 passed all three native platforms without
publishing. Native Mac handoff/rollback passed at 6.46/4.34 seconds on Apple
Silicon and 18.94/7.65 seconds on Intel; these are isolated fixture timings,
not user-facing latency guarantees. The final tag additionally checks actual
serving kernel identity and includes the Settings browser fixture on every
platform. The valid ZIP is created/extracted using the same ditto path as
release packages, and a detached copy of the real application performs success
handoff. The launch-failure boundary is injected for deterministic rollback.
No real user configuration, Key or cloud inference is touched by these tests.

The v0.1.113 tag was not published: official Apple Silicon run
44041856030 detected a cleanup race after successful helper assertions because
the isolated kernel was still writing the temporary HOME. The regression now
waits for the exact updated Runtime and isolated kernel to exit before removing
the fixture. No production user process is targeted. This fix is carried in
v0.1.114 rather than moving/reusing the failed version tag.

## Publication

Release v0.1.114 published at 2026-11-17T08:20:31Z from revision `ac4b95f`.
Workflow 44043479604 passed all three platform jobs and publication, including
Mac native helper success/rollback and clean fixture shutdown. Apple Silicon
success/rollback took 7.56/5.26 seconds, Intel 22.16/9.67 seconds in isolated
fixtures. Windows upgrade, quiet Codex routing, native login renewal and
packaged-kernel compatibility remained passing. Settings browser checks passed
both themes and 390/1440 widths; the duplicate Agent section is absent.

The public release resolver returns 0.1.114 with all three mirrors. Five final
installer/DMG/ZIP assets are checked against published SHA256SUMS. No cloud
application, profile, price book or user credentials were changed by this release.
Mac users on earlier versions must manually install this release once to gain
future in-app updates; current system permissions and security prompts remain.
